package cgroup

import (
	"testing"
)

func TestCgroup_Name(t *testing.T) {
	tests := []struct {
		name string
		cg   *Cgroup
		want string
	}{
		{
			name: "Test with name 'test-cgroup'",
			cg:   &Cgroup{name: "test-cgroup"},
			want: "test-cgroup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cg.Name(); got != tt.want {
				t.Errorf("Cgroup.Name() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCgroup_SetMemoryLimit(t *testing.T) {
	tests := []struct {
		name string
		cg   *Cgroup
		mb   int
	}{
		{
			name: "Set memory limit to 512MB",
			cg:   &Cgroup{},
			mb:   512,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cg.SetMemoryLimit(tt.mb)
			if got := tt.cg.MemoryLimit(); got != tt.mb {
				t.Errorf("Cgroup.SetMemoryLimit() = %v, want %v", got, tt.mb)
			}
		})
	}
}

func TestCgroup_AddPid(t *testing.T) {
	tests := []struct {
		name    string
		cg      *Cgroup
		pid     string
		wantErr bool
	}{
		{
			name:    "Add valid PID",
			cg:      &Cgroup{},
			pid:     "1234",
			wantErr: false,
		},
		{
			name:    "Add invalid PID",
			cg:      &Cgroup{},
			pid:     "invalid-pid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cg.AddPid(tt.pid); (err != nil) != tt.wantErr {
				t.Errorf("Cgroup.AddPid() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
