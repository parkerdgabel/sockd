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
	IsLeaf           bool
	Installs         []string
	Imports          []string
	Runtime          Runtime
	MemLimitMB       int
	CPUPercent       int
	BaseImageName    string
	BaseImageVersion string
}
