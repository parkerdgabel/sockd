package container

type Runtime string // Runtime is the container runtime to use

const (
	Python Runtime = "python"
	Node   Runtime = "node"
	Go     Runtime = "go"
	Java   Runtime = "java"
	Ruby   Runtime = "ruby"
)

type Meta struct {
	ParentID         string
	Installs         []string
	Imports          []string
	Runtime          Runtime
	MemLimitMB       int
	CPUPercent       int
	BaseImageName    string
	BaseImageVersion string
	isLeaf           bool
}

func (m *Meta) IsZgote() bool {
	return !m.isLeaf
}

func (m *Meta) MakeZygote() *Meta {
	m.isLeaf = false
	return m
}
