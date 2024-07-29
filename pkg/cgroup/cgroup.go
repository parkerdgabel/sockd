package cgroup

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Cgroup struct {
	name       string
	pool       *Pool
	memLimitMB int
}

func (cg *Cgroup) printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s [CGROUP %s: %s]", strings.TrimRight(msg, "\n"), cg.pool.Name, cg.name)

}

// ResourcePath returns the path to a specific resource in this cgroup
func (cg *Cgroup) ResourcePath(resource string) string {
	return fmt.Sprintf("%s/%s/%s", cg.pool.GroupPath(), cg.name, resource)
}

// Name returns the name of the cgroup
func (cg *Cgroup) Name() string {
	return cg.name
}

// SetMemoryLimit sets the memory limit for the cgroup
func (cg *Cgroup) SetMemoryLimit(mb int) {
	cg.memLimitMB = mb
}

func (cg *Cgroup) Release() {
	// if there's room in the recycled channel, add it there.
	// Otherwise, just delete it.
	for i := 100; i >= 0; i-- {
		pids, err := cg.GetPIDs()
		if err != nil {
			panic(err)
		} else if len(pids) > 0 {
			if i == 0 {
				panic(fmt.Errorf("Cannot release cgroup that contains processes: %v", pids))
			}

			cg.printf("cgroup Rmdir failed, trying again in 5ms")
			time.Sleep(5 * time.Millisecond)
		} else {
			break
		}
	}

	select {
	case cg.pool.recycled <- cg:
		cg.printf("release and recycle")
		return
	default:
	}

	cg.printf("release and Destroy")
	cg.Destroy()
}

// Destroy this cgroup
func (cg *Cgroup) Destroy() {
	gpath := cg.GroupPath()
	cg.printf("Destroying cgroup with path \"%s\"", gpath)

	for i := 100; i >= 0; i-- {
		if err := syscall.Rmdir(gpath); err != nil {
			if i == 0 {
				panic(fmt.Errorf("Rmdir(2) %s: %s", gpath, err))
			}

			cg.printf("cgroup Rmdir failed, trying again in 5ms")
			time.Sleep(5 * time.Millisecond)
		} else {
			break
		}
	}
}

// MemoryLimit returns the memory limit for the cgroup
func (cg *Cgroup) MemoryLimit() int {
	return cg.memLimitMB
}

func (cg *Cgroup) TryWriteInt(resource string, val int64) error {
	return os.WriteFile(cg.ResourcePath(resource), []byte(fmt.Sprintf("%d", val)), os.ModeAppend)
}

func (cg *Cgroup) TryWriteString(resource string, val string) error {
	return os.WriteFile(cg.ResourcePath(resource), []byte(val), os.ModeAppend)
}

func (cg *Cgroup) WriteInt(resource string, val int64) {
	if err := cg.TryWriteInt(resource, val); err != nil {
		panic(fmt.Sprintf("Error writing %v to %s: %v", val, resource, err))
	}
}

func (cg *Cgroup) WriteString(resource string, val string) {
	if err := cg.TryWriteString(resource, val); err != nil {
		panic(fmt.Sprintf("Error writing %v to %s: %v", val, resource, err))
	}
}

func (cg *Cgroup) TryReadIntKV(resource string, key string) (int64, error) {
	raw, err := os.ReadFile(cg.ResourcePath(resource))
	if err != nil {
		return 0, err
	}
	body := string(raw)
	lines := strings.Split(body, "\n")
	for i := 0; i <= len(lines); i++ {
		parts := strings.Split(lines[i], " ")
		if len(parts) == 2 && parts[0] == key {
			val, err := strconv.ParseInt(strings.TrimSpace(string(parts[1])), 10, 64)
			if err != nil {
				return 0, err
			}
			return val, nil
		}
	}
	return 0, fmt.Errorf("could not find key '%s' in file: %s", key, body)
}

func (cg *Cgroup) TryReadInt(resource string) (int64, error) {
	raw, err := os.ReadFile(cg.ResourcePath(resource))
	if err != nil {
		return 0, err
	}
	val, err := strconv.ParseInt(strings.TrimSpace(string(raw)), 10, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func (cg *Cgroup) ReadInt(resource string) int64 {
	val, err := cg.TryReadInt(resource)

	if err != nil {
		panic(err)
	}

	return val
}

func (cg *Cgroup) AddPid(pid string) error {
	err := os.WriteFile(cg.ResourcePath("cgroup.procs"), []byte(pid), os.ModeAppend)
	if err != nil {
		return err
	}

	return nil
}

func (cg *Cgroup) setFreezeState(state int64) error {
	cg.WriteInt("cgroup.freeze", state)

	timeout := 5 * time.Second

	start := time.Now()
	for {
		freezerState, err := cg.TryReadInt("cgroup.freeze")
		if err != nil {
			return fmt.Errorf("failed to check self_freezing state :: %v", err)
		}

		if freezerState == state {
			return nil
		}

		if time.Since(start) > timeout {
			return fmt.Errorf("cgroup stuck on %v after %v (should be %v)", freezerState, timeout, state)
		}

		time.Sleep(1 * time.Millisecond)
	}
}

// get mem usage in MB
func (cg *Cgroup) GetMemUsageMB() int {
	usage := cg.ReadInt("memory.current")

	// round up to nearest MB
	mb := int64(1024 * 1024)
	return int((usage + mb - 1) / mb)
}

// get mem limit in MB
func (cg *Cgroup) GetMemLimitMB() int {
	return cg.memLimitMB
}

// set mem limit in MB
func (cg *Cgroup) SetMemLimitMB(mb int) {
	if mb == cg.memLimitMB {
		return
	}

	limitPath := cg.ResourcePath("memory.max")
	bytes := int64(mb) * 1024 * 1024
	cg.WriteInt("memory.max", bytes)

	// cgroup v1 documentation recommends reading back limit after
	// writing, because it is only a suggestion (e.g., may get
	// rounded to page size).
	//
	// we don't have a great way of dealing with this now, so
	// we'll just panic if it is not within some tolerance
	limitRaw, err := os.ReadFile(limitPath)
	if err != nil {
		panic(err)
	}
	limit, err := strconv.ParseInt(strings.TrimSpace(string(limitRaw)), 10, 64)
	if err != nil {
		panic(err)
	}

	diff := limit - bytes
	if diff < -1024*1024 || diff > 1024*1024 {
		panic(fmt.Errorf("tried to set mem limit to %d, but result (%d) was not within 1MB tolerance",
			bytes, limit))
	}

	cg.memLimitMB = mb
}

// percent of a core
func (cg *Cgroup) SetCPUPercent(percent int) {
	period := 100000 // 100 ms
	quota := period * percent / 100
	cg.WriteString("cpu.max", fmt.Sprintf("%d %d", quota, period))
}

// Freeze processes in the cgroup
func (cg *Cgroup) Pause() error {
	return cg.setFreezeState(1)
}

// Unfreeze processes in the cgroup
func (cg *Cgroup) Unpause() error {
	return cg.setFreezeState(0)
}

// Get the IDs of all processes running in this cgroup
func (cg *Cgroup) GetPIDs() ([]string, error) {
	procsPath := cg.ResourcePath("cgroup.procs")
	pids, err := os.ReadFile(procsPath)
	if err != nil {
		return nil, err
	}

	pidStr := strings.TrimSpace(string(pids))
	if len(pidStr) == 0 {
		return []string{}, nil
	}

	return strings.Split(pidStr, "\n"), nil
}

func (cg *Cgroup) CgroupProcsPath() string {
	return cg.ResourcePath("cgroup.procs")
}

// KillAllProcs stops all processes inside the cgroup
// Note, the CG most be paused beforehand
func (cg *Cgroup) KillAllProcs() {
	cg.WriteInt("cgroup.kill", 1)
}

// GroupPath returns the path to the Cgroup pool for OpenLambda
func (cg *Cgroup) GroupPath() string {
	return fmt.Sprintf("%s/%s", cg.pool.GroupPath(), cg.name)
}
