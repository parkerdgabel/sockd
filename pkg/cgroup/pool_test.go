package cgroup

import (
	"log"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func TestPool_NewPool(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "Test with name 'test-pool'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewPool("test-pool")
			if err != nil {
				t.Errorf("NewPool() error = %v", err)
			}
			pool.Destroy()
		})
	}
}

func TestPool_Destroy(t *testing.T) {
	pool, err := NewPool("test-pool")

	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}

	pool.Destroy()

	// Check if the pool is properly destroyed
	// This is a placeholder as the actual implementation might vary
	if _, err := os.Stat(pool.GroupPath()); !os.IsNotExist(err) {
		t.Errorf("Destroy() did not remove the cgroup pool")
	}
}

func TestPool_GroupPath(t *testing.T) {
	pool, err := NewPool("test-pool")

	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Destroy()

	expectedPath := path.Join(CgroupPath, "cgroup-test-pool")
	if pool.GroupPath() != expectedPath {
		t.Errorf("GroupPath() = %v, want %v", pool.GroupPath(), expectedPath)
	}
}

func TestPool_RetrieveCgroup(t *testing.T) {
	pool, err := NewPool("test-pool")
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Destroy()

	timeout := time.Duration(1) * time.Second
	cgroup, err := pool.RetrieveCgroup(timeout)
	if err != nil {
		t.Fatalf("RetrieveCgroup() error = %v", err)
	}

	if cgroup.pool != pool {
		t.Errorf("RetrieveCgroup() returned a cgroup with the wrong pool")
	}
	cgroup.Release()
}

func TestPool_printf(t *testing.T) {
	pool, err := NewPool("test-pool")
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Destroy()

	// Redirect log output for testing
	var logOutput strings.Builder
	log.SetOutput(&logOutput)
	defer log.SetOutput(os.Stderr)

	pool.printf("test message %d", 1)

	if !strings.Contains(logOutput.String(), "test message 1") {
		t.Errorf("printf() did not log the expected message")
	}
}
