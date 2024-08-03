package cgroup

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"syscall"
	"time"
)

type CgroupPoolError struct {
	resource string
	err      error
}

func (e *CgroupPoolError) Error() string {
	return "CgroupPool error: " + e.resource + ": " + e.err.Error()
}

// if there are fewer than CGROUP_RESERVE available, more will be created.
// If there are more than 2*CGROUP_RESERVE available, they'll be released.
const (
	CGROUP_RESERVE = 16
	SubTreeControl = "cgroup.subtree_control"
	Controller     = "cgroup.controller"
	CgroupPath     = "/sys/fs/cgroup"
	Controllers    = "+pids +io +memory +cpu"
)

type Pool struct {
	Name     string
	ready    chan *Cgroup
	recycled chan *Cgroup
	quit     chan chan bool
	nextID   int
}

// NewPool creates a new Cgroup pool
func NewPool(name string) (*Pool, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("Getwd: %s", err)
	}
	pool := &Pool{
		Name:     path.Base(wd) + "-" + name,
		ready:    make(chan *Cgroup, CGROUP_RESERVE),
		recycled: make(chan *Cgroup, CGROUP_RESERVE),
		quit:     make(chan chan bool),
		nextID:   0,
	}

	// create cgroup
	groupPath := pool.GroupPath()
	pool.printf("create %s", groupPath)
	if err := syscall.Mkdir(groupPath, 0700); err != nil {
		return nil, &CgroupPoolError{"Mkdir", err}
	}

	// Make controllers available to child groups
	rpath := fmt.Sprintf("%s/%s", groupPath, SubTreeControl)
	if err := os.WriteFile(rpath, []byte(Controllers), os.ModeAppend); err != nil {
		return nil, &CgroupPoolError{"WriteFile", err}
	}
	go pool.cgTask()

	return pool, nil
}

// NewCgroup creates a new CGroup in the pool
func (pool *Pool) NewCgroup() (*Cgroup, error) {
	pool.nextID++

	cg := &Cgroup{
		name: fmt.Sprintf("cg-%d", pool.nextID),
		pool: pool,
	}

	groupPath := cg.GroupPath()
	if err := os.Mkdir(groupPath, 0700); err != nil {
		return nil, &CgroupError{"Mkdir", err}
	}

	cg.printf("created")
	return cg, nil
}

func (pool *Pool) cgTask() {
	// we'll be sent this as part of the quit request
	var done chan bool

	// loop until we get the quit message
	pool.printf("start creating/serving CGs")
Loop:
	for {
		var cg *Cgroup

		// get a new or recycled cgroup.  Settings may be initialized
		// in one of three places, the first two of which are here:
		//
		// 1. upon fresh creation (things that never change, such as max procs)
		// 2. after it's been recycled (we need to clean things up that change during use)
		// 3. some things (e.g., memory limits) need to be done in either case, and may
		//    depend on the needs of the Sandbox; this happens in pool.GetCg (which is
		//    fed by this function)
		select {
		case cg = <-pool.recycled:
			// restore cgroup to clean state
			// FIXME not possible in CG2?
			// cg.WriteInt("memory.failcnt", 0)
			if err := cg.Unpause(); err != nil {
				cg.printf("Unpause failed: %s", err)
			}
		default:
			// t := common.T0("fresh-cgroup")
			cg, _ = pool.NewCgroup()
			// TODO: set up Config for max procs, memory limits, etc.
			cg.WriteInt("pids.max", int64(10))
			cg.WriteInt("memory.swap.max", int64(0))
			// t.T1()
		}

		// add cgroup to ready queue
		select {
		case pool.ready <- cg:
		case done = <-pool.quit:
			pool.printf("received shutdown request")
			cg.Destroy()
			break Loop
		}
	}

	// empty queues, freeing all cgroups
	pool.printf("empty queues and release CGs")
Empty:
	for {
		select {
		case cg := <-pool.ready:
			cg.Destroy()
		case cg := <-pool.recycled:
			cg.Destroy()
		default:
			break Empty
		}
	}

	done <- true
}

// RetrieveCg retrieves a Cgroup from the pool with a timeout
func (pool *Pool) RetrieveCgroup(timeout time.Duration) (*Cgroup, error) {
	select {
	case cg := <-pool.ready:
		return cg, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("timeout waiting to retrieve Cgroup")
	}
}

// Destroy this entire cgroup pool
func (pool *Pool) Destroy() error {
	// signal cgTask, then wait for it to finish
	ch := make(chan bool)
	pool.quit <- ch
	<-ch

	// Destroy cgroup for this entire pool
	gpath := pool.GroupPath()
	pool.printf("Destroying cgroup pool with path \"%s\"", gpath)
	for i := 100; i >= 0; i-- {
		if err := syscall.Rmdir(gpath); err != nil {
			if i == 0 {
				return &CgroupPoolError{resource: gpath, err: err}
			}

			pool.printf("cgroup pool Rmdir failed, trying again in 5ms")
			time.Sleep(5 * time.Millisecond)
		} else {
			break
		}
	}
	return nil
}

// add ID to each log message so we know which logs correspond to
// which containers
func (pool *Pool) printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s [CGROUP POOL %s]", strings.TrimRight(msg, "\n"), pool.Name)
}

// GroupPath returns the path to the Cgroup pool for OpenLambda
func (pool *Pool) GroupPath() string {
	return fmt.Sprintf("/sys/fs/cgroup/%s", pool.Name)
}
