package zygote

import (
	"container/list"
	"fmt"
	"log"
	"parkerdgabel/sockd/pkg/container"
	"strings"
)

const (
	FreeContainerPercentGoal = 20
	ConcurrentEvictions      = 8
)

type Evictor struct {
	mem    *MemPool
	events chan container.ContainerEvent
	// Sandbox ID => prio.  we ALWAYS evict lower priority before higher priority
	//
	// A Sandbox's priority is 2*NUM_CHILDEN, +1 if Unpaused.
	// Thus, we'll prefer to evict paused (idle) sandboxes with no
	// children.  Under pressure, we'll evict running sandboxes
	// (this will surface an error to the end user).  We'll never
	// invoke from priority 2+ (i.e., those with at least one
	// child), as there is no benefit to evicting Sandboxes with
	// live children (we can't reclaim memory until all
	// descendents exit)
	priority map[string]int
	// state queues (each Sandbox is on at most one of these)
	prioQueues []*list.List
	evicting   *list.List

	// Sandbox ID => List/Element position in a state queue
	stateMap map[string]*ListLocation
}

type ListLocation struct {
	*list.List
	*list.Element
}

func NewEvictor(provider Provider) *Evictor {
	evictor := &Evictor{
		mem:        provider.MemPool(),
		events:     make(chan container.ContainerEvent, 32),
		priority:   make(map[string]int),
		prioQueues: make([]*list.List, 3),
		stateMap:   make(map[string]*ListLocation),
	}

	for i := 0; i < 3; i++ {
		evictor.prioQueues[i] = list.New()
	}

	go evictor.run()
	return evictor
}

func (evictor *Evictor) run() {
	// map container ID to the element that is on one of the lists

	for {
		// blocks until there's at least one update
		evictor.updateState()

		// select 0 or more sandboxes to evict (policy), then
		// .Destroy them (mechanism)
		evictor.doEvictions()
	}
}

// POLICY: how should we select a victim?
func (evictor *Evictor) doEvictions() {
	// TODO: consider a more sophisticated policy
	memLimitMB := evictor.mem.totalMB / 2

	// how many sandboxes could we spin up, given available mem?
	freeSandboxes := evictor.mem.getAvailableMB() / memLimitMB

	// how many sandboxes would we like to be able to spin up,
	// without waiting for more memory?
	freeGoal := 1 + ((evictor.mem.totalMB/memLimitMB)-2)*FreeContainerPercentGoal/100

	// how many shoud we try to evict?
	//
	// TODO: consider counting in-flight evictions.  This will be
	// a bit tricky, as the evictions may be of sandboxes in paused
	// states with reduced memory limits
	evictCount := freeGoal - freeSandboxes

	evictCap := ConcurrentEvictions - evictor.evicting.Len()
	if evictCap < evictCount {
		evictCount = evictCap
	}

	// try evicting the desired number, starting with the paused queue
	for evictCount > 0 && evictor.prioQueues[0].Len() > 0 {
		evictor.evictFront(evictor.prioQueues[0], false)
		evictCount -= 1
	}

	// we don't like to evict running containers, because that
	// interrupts requests, but we do if necessary to keep the
	// system moving (what if all lambdas hanged forever?)
	//
	// TODO: create some parameters to better control eviction in
	// this state
	if freeSandboxes <= 0 && evictor.evicting.Len() == 0 {
		evictor.printf("WARNING!  Critically low on memory, so evicting an active Sandbox")
		if evictor.prioQueues[1].Len() > 0 {
			evictor.evictFront(evictor.prioQueues[1], true)
		}
	}

	// we never evict from prioQueues[2+], because those have
	// descendents with lower priority that should be evicted
	// first
}

// evict whatever SB is at the front of the queue, assumes
// queue is not empty
func (evictor *Evictor) evictFront(queue *list.List, force bool) {
	front := queue.Front()
	sb := front.Value.(*container.Container)

	evictor.printf("Evict Sandbox %v", sb.ID())
	evictor.move(sb, evictor.evicting)

	// destroy async (we'll know when it's done, because
	// we'll see a evDestroy event later on our chan)
	go func() {

		if force {
			sb.Destroy()
		} else {
			// sb.DestroyIfPaused("idle eviction")
		}

	}()
}

// update state based on messages sent to this task.  this may be
// stale, but correctness doesn't depend on freshness.
//
// blocks until there's at least one event
func (evictor *Evictor) updateState() {
	event := evictor.nextEvent(true)

	// update state based on incoming messages
	for event != nil {
		// add list to appropriate queue
		c := event.Container
		prio := evictor.priority[c.ID()]

		switch event.Event {
		case container.ContainerStart:
			if prio != 0 {
				panic(fmt.Sprintf("Sandboxes should be at prio 0 upon EvCreate event but it was %d for %d", prio, c.ID()))
			}
			prio += 1
		case container.ContainerUnpause:
			prio += 1
		case container.ContainerPause:
			prio -= 1
		case container.ContainerFork:
			prio += 2
		case container.ContainerChildExit:
			prio -= 2
		case container.ContainerDestroy:
		default:
			evictor.printf("Unknown event: %v", event.Event)
		}

		evictor.printf("Evictor: Sandbox %v priority goes to %d", c.ID(), prio)
		if prio < 0 {
			panic(fmt.Sprintf("priority should never go negative, but it went to %d for sandbox %d", prio, c.ID()))

		}

		if event.Event == container.ContainerDestroy {
			evictor.move(c, nil)
			delete(evictor.priority, c.ID())
		} else {
			evictor.priority[c.ID()] = prio
			// saturate prio based on number of queues
			if prio >= len(evictor.prioQueues) {
				prio = len(evictor.prioQueues) - 1
			}

			evictor.move(c, evictor.prioQueues[prio])
		}

		event = evictor.nextEvent(false)
	}
}

func (evictor *Evictor) nextEvent(block bool) *container.ContainerEvent {
	if block {
		event := <-evictor.events
		return &event
	}

	select {
	case event := <-evictor.events:
		return &event
	default:
		return nil
	}
}

// move Sandbox to a given queue, removing from previous (if necessary).
// a move to nil is just a delete.
func (evictor *Evictor) move(c *container.Container, target *list.List) {
	// remove from previous queue if necessary
	prev := evictor.stateMap[c.ID()]
	if prev != nil {
		prev.List.Remove(prev.Element)
	}

	// add to new queue
	if target != nil {
		element := target.PushBack(c)
		evictor.stateMap[c.ID()] = &ListLocation{target, element}
	} else {
		delete(evictor.stateMap, c.ID())
	}
}

func (_ *Evictor) printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s [EVICTOR]", strings.TrimRight(msg, "\n"))

}
