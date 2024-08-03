package cgroup

import "testing"

func Test_CgroupGroupPath(t *testing.T) {
	pool, err := NewPool("test-pool")
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Destroy()
	tests := []struct {
		name     string
		cgroup   *Cgroup
		expected string
	}{
		{
			name:     "test-pool",
			cgroup:   &Cgroup{pool: pool, name: "test"},
			expected: "/sys/fs/cgroup/cgroup-test-pool/test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cgroup.GroupPath() != tt.expected {
				t.Errorf("GroupPath() = %v, want %v", tt.cgroup.GroupPath(), tt.expected)
			}
		})
	}

}
