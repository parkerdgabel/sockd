package container

import (
	"os"
	"parkerdgabel/sockd/pkg/cgroup"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func setupDirs(t *testing.T) (string, string, string, string, func()) {
	baseDir, err := os.MkdirTemp("", "baseDir")
	if err != nil {
		t.Fatalf("failed to create temp baseDir: %v", err)
	}
	// add handler directory to baseDIr
	handlerDir := filepath.Join(baseDir, "handler")
	if err := os.Mkdir(handlerDir, 0755); err != nil {
		t.Fatalf("failed to create handler directory: %v", err)
	}
	hostDir := filepath.Join(baseDir, "host")
	if err := os.Mkdir(hostDir, 0755); err != nil {
		t.Fatalf("failed to create handler directory: %v", err)
	}
	tmpDir := filepath.Join(baseDir, "tmp")
	if err := os.Mkdir(tmpDir, 0755); err != nil {
		t.Fatalf("failed to create handler directory: %v", err)
	}
	rootDir, err := os.MkdirTemp("", "rootDir")
	if err != nil {
		t.Fatalf("failed to create temp rootDir: %v", err)
	}
	codeDir, err := os.MkdirTemp("", "codeDir")
	if err != nil {
		t.Fatalf("failed to create temp codeDir: %v", err)
	}
	scratchDir, err := os.MkdirTemp("", "scratchDir")
	if err != nil {
		t.Fatalf("failed to create temp scratchDir: %v", err)
	}
	return baseDir, rootDir, codeDir, scratchDir, func() {
		// dirs := []string{handlerDir, hostDir, tmpDir, rootDir, codeDir, scratchDir, baseDir}
		// for _,  := range dirs {
		// 	// // Attempt to remount the directory as read-write
		// 	// if err := syscall.Mount("", dir, "", syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
		// 	// 	t.Logf("failed to remount directory %s as read-write: %v", dir, err)
		// 	// }
		// 	// Change permissions to ensure we can remove the directory
		// 	// if err := os.Chmod(dir, 0700); err != nil {
		// 	// 	t.Logf("failed to change permissions for directory %s: %v", dir, err)
		// 	// }
		// 	// Attempt to remove the directory
		// 	if err := os.RemoveAll(dir); err != nil {
		// 		t.Fatalf("failed to remove directory %s: %v", dir, err)
		// 	}
		// }
	}
}

func TestNewContainer(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	t.Cleanup(func() {
		teardown()
	})

	cgroup := &cgroup.Cgroup{}
	meta := &Meta{
		Runtime: Python,
	}
	container, err := NewContainer(nil, baseDir, "test-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if container.ID() != "test-id" {
		t.Errorf("expected id to be 'test-id', got %v", container.ID())
	}

}

func TestContainerStartClient(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	t.Cleanup(func() {
		teardown()
	})

	cgroup := &cgroup.Cgroup{}
	meta := &Meta{
		Runtime: Python,
	}
	container, err := NewContainer(nil, baseDir, "test-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.StartClient()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerDestroy(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	cgroupPool, err := cgroup.NewPool("test-pool")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	t.Cleanup(func() {
		teardown()
		cgroupPool.Destroy()
	})

	cgroup, err := cgroupPool.RetrieveCgroup(time.Duration(1) * time.Second)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	meta := &Meta{
		Runtime: Python,
	}
	container, err := NewContainer(nil, baseDir, "test-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Destroy()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	cgroup.Destroy()
}

func TestContainerStart(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	t.Cleanup(func() {
		teardown()
	})

	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, baseDir, "test-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Start()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerPause(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	t.Cleanup(func() {
		teardown()
	})

	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, baseDir, "test-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Pause()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerUnpause(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	t.Cleanup(func() {
		teardown()
	})

	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, baseDir, "test-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = container.Unpause()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerFork(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	t.Cleanup(func() {
		teardown()
	})

	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	parent, err := NewContainer(nil, baseDir, "parent-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	child, err := NewContainer(nil, baseDir, "child-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = parent.Fork(child)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestContainerMountScratchDir(t *testing.T) {
	baseDir, rootDir, codeDir, scratchDir, teardown := setupDirs(t)
	t.Cleanup(func() {
		teardown()
	})

	cgroup := &cgroup.Cgroup{}
	meta := &Meta{}
	container, err := NewContainer(nil, baseDir, "test-id", rootDir, codeDir, scratchDir, cgroup, meta, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	sbScratchDir := filepath.Join(container.rootDir, "host")
	if err := syscall.Mount(container.scratchDir, sbScratchDir, "", syscall.MS_BIND, ""); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
