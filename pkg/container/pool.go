package container

import (
	"parkerdgabel/sockd/pkg/cgroup"
	"parkerdgabel/sockd/pkg/mem"
)

type Pool struct {
	name       string
	mem        *mem.MemPool
	cgroupPool *cgroup.Pool
}

func NewPool(name string, totalMB int) *Pool {
	cgroupPool, err := cgroup.NewPool(name)
	if err != nil {
		return nil
	}
	return &Pool{
		name:       name,
		mem:        mem.NewMemPool(name, totalMB),
		cgroupPool: cgroupPool,
	}
}
