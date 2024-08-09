package container

import (
	"parkerdgabel/sockd/pkg/cgroup"
	"path/filepath"
	"syscall"
	"testing"
)

func TestNewContainer(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if container.ID() != "test-id" {
		t.Errorf("expected id to be 'test-id', got %v", container.ID())
	}
}

func TestContainerStartClient(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "/base/image/dir", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.StartClient()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerStopClient(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "/base/image/dir", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	container.StartClient()
	container.StopClient()
	if container.client != nil {
		t.Errorf("expected client to be nil, got %v", container.client)
	}
}

func TestContainerDestroy(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "/base/image/dir", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Destroy()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerStart(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "/base/image/dir", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Start()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerPause(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "/base/image/dir", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Pause()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerUnpause(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "/base/image/dir", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Unpause()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerFork(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	parent, err := NewContainer(nil, "/base/image/dir", "parent-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	child, err := NewContainer(nil, "/base/image/dir", "child-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = parent.Fork(child)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerMountScratchDir(t *testing.T) {
	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, "/base/image/dir", "test-id", "/root/dir", "/code/dir", "/scratch/dir", cgroup, meta)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	sbScratchDir := filepath.Join(container.rootDir, "host")
	if err := syscall.Mount(container.scratchDir, sbScratchDir, "", BIND, ""); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
