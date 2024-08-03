package container

import "parkerdgabel/sockd/pkg/cgroup"

type Runtime string

const (
	Java   Runtime = "java"
	Python Runtime = "python"
	Node   Runtime = "node"
	Go     Runtime = "go"
	Ruby   Runtime = "ruby"
)

type Container struct {
	id         string
	meta       *Meta
	runtime    Runtime
	rootDir    string
	codeDir    string
	scratchDir string
	cgroup     *cgroup.Cgroup
	// 1 for self, plus 1 for each child (we can't release memory
	// until all descendants are dead, because they share the
	// pages of this Container, but this is the only container
	// charged)
	cgRefCount int32
	parent     *Container
	children   map[string]*Container
}

func NewContainer(id string, meta *Meta, runtime Runtime, rootDir string, codeDir string, scratchDir string, cgroup *cgroup.Cgroup) *Container {
	return &Container{
		id:         id,
		meta:       meta,
		runtime:    runtime,
		rootDir:    rootDir,
		codeDir:    codeDir,
		scratchDir: scratchDir,
		cgroup:     cgroup,
		children:   make(map[string]*Container),
	}
}

func (c *Container) ID() string {
	return c.id
}

func (c *Container) Meta() *Meta {
	return c.meta
}

func (c *Container) Runtime() Runtime {
	return c.runtime
}

func (c *Container) RootDir() string {
	return c.rootDir
}

func (c *Container) CodeDir() string {
	return c.codeDir
}

func (c *Container) ScratchDir() string {
	return c.scratchDir
}

func (c *Container) Cgroup() *cgroup.Cgroup {
	return c.cgroup
}

func (c *Container) Parent() *Container {
	return c.parent
}

func (c *Container) Children() map[string]*Container {
	return c.children
}

func (c *Container) AddChild(child *Container) {
	c.children[child.ID()] = child
	child.parent = c
}

func (c *Container) RemoveChild(child *Container) {
	delete(c.children, child.ID())
	child.parent = nil
}
