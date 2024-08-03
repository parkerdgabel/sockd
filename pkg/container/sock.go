package container

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"parkerdgabel/sockd/pkg/cgroup"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type ContainerError struct {
	container string
	err       error
}

func (e *ContainerError) Error() string {
	return "Container error: " + e.container + ": " + e.err.Error()
}

type Runtime string

const (
	Java   Runtime = "java"
	Python Runtime = "python"
	Node   Runtime = "node"
	Go     Runtime = "go"
	Ruby   Runtime = "ruby"
)

var BIND uintptr = uintptr(syscall.MS_BIND)
var BIND_RO uintptr = uintptr(syscall.MS_BIND | syscall.MS_RDONLY | syscall.MS_REMOUNT)
var PRIVATE uintptr = uintptr(syscall.MS_PRIVATE)
var SHARED uintptr = uintptr(syscall.MS_SHARED)

type Container struct {
	id         string
	meta       *Meta
	runtime    Runtime
	rootDir    string
	codeDir    string
	scratchDir string
	cgroup     *cgroup.Cgroup
	client     *http.Client
	// 1 for self, plus 1 for each child (we can't release memory
	// until all descendants are dead, because they share the
	// pages of this Container, but this is the only container
	// charged)
	cgRefCount int32
	parent     *Container
	children   map[string]*Container
}

func NewContainer(id string, meta *Meta, runtime Runtime, rootDir, codeDir, scratchDir string, cgroup *cgroup.Cgroup) *Container {
	return &Container{
		id:         id,
		meta:       meta,
		runtime:    runtime,
		rootDir:    rootDir,
		codeDir:    codeDir,
		scratchDir: scratchDir,
		cgroup:     cgroup,
		client:     &http.Client{},
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

func (c *Container) Client() *http.Client {
	return c.client
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

func (c *Container) StartClient() error {
	sockPath := filepath.Join(c.scratchDir, "reactor.sock")
	if len(sockPath) > 108 {
		return &ContainerError{container: c.id, err: fmt.Errorf("socket path length cannot exceed 108 characters (try moving cluster closer to the root directory")}
	}
	c.printf("starting client with socket path: %s", sockPath)
	dial := func(proto, addr string) (net.Conn, error) {
		return net.Dial("unix", sockPath)
	}

	c.client = &http.Client{
		Transport: &http.Transport{Dial: dial},
		Timeout:   time.Second * time.Duration(3), // TODO make this configurable
	}
	return nil
}

func (c *Container) Pause() error {
	if err := c.cgroup.Pause(); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	oldLimit := c.cgroup.GetMemLimitMB()
	newLimit := c.cgroup.GetMemUsageMB() + 1
	if newLimit < oldLimit {
		c.cgroup.SetMemLimitMB(newLimit)
		c.pool.mem.adjustAvailableMB(oldLimit - newLimit)
	}
	return nil
}

func (c *Container) populateRoot(baseDir string) error {
	if err := syscall.Mount(baseDir, c.rootDir, "", BIND, ""); err != nil {
		return &ContainerError{container: c.id, err: fmt.Errorf("failed to bind root dir: %s -> %s :: %v", baseDir, c.rootDir, err)}
	}

	if err := syscall.Mount("none", c.rootDir, "", BIND_RO, ""); err != nil {
		return &ContainerError{container: c.id, err: fmt.Errorf("failed to bind root dir RO: %s :: %v", c.rootDir, err)}
	}

	if err := syscall.Mount("none", c.rootDir, "", PRIVATE, ""); err != nil {
		return &ContainerError{container: c.id, err: fmt.Errorf("failed to make root dir private :: %v", err)}
	}

	// FILE SYSTEM STEP 2: code dir
	if c.codeDir != "" {
		sbCodeDir := filepath.Join(c.rootDir, "handler")

		if err := syscall.Mount(c.codeDir, sbCodeDir, "", BIND, ""); err != nil {
			return &ContainerError{container: c.id, err: fmt.Errorf("Failed to bind code dir: %s -> %s :: %v", c.codeDir, sbCodeDir, err.Error())}
		}

		if err := syscall.Mount("none", sbCodeDir, "", BIND_RO, ""); err != nil {
			return &ContainerError{container: c.id, err: fmt.Errorf("failed to bind code dir RO: %v", err.Error())}
		}
	}

	// FILE SYSTEM STEP 3: scratch dir (tmp and communication)
	tmpDir := filepath.Join(c.scratchDir, "tmp")
	if err := os.Mkdir(tmpDir, 0777); err != nil && !os.IsExist(err) {
		return &ContainerError{container: c.id, err: err}
	}

	sbScratchDir := filepath.Join(c.rootDir, "host")
	if err := syscall.Mount(c.scratchDir, sbScratchDir, "", BIND, ""); err != nil {
		return &ContainerError{container: c.id, err: fmt.Errorf("failed to bind scratch dir: %v", err.Error())}
	}

	// TODO: cheaper to handle with symlink in lambda image?
	sbTmpDir := filepath.Join(c.rootDir, "tmp")
	if err := syscall.Mount(tmpDir, sbTmpDir, "", BIND, ""); err != nil {
		return &ContainerError{container: c.id, err: fmt.Errorf("failed to bind tmp dir: %v", err.Error())}
	}

	return nil
}

// add ID to each log message so we know which logs correspond to
// which containers
func (c *Container) printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s [SOCK %s]", strings.TrimRight(msg, "\n"), c.id)
}
