package container

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"parkerdgabel/sockd/pkg/cgroup"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type ContainerError struct {
	container string
	err       error
}

func (e *ContainerError) Error() string {
	return "Container error: " + e.container + ": " + e.err.Error()
}

var BIND uintptr = uintptr(syscall.MS_BIND)
var BIND_RO uintptr = uintptr(syscall.MS_BIND | syscall.MS_RDONLY | syscall.MS_REMOUNT)
var PRIVATE uintptr = uintptr(syscall.MS_PRIVATE)
var SHARED uintptr = uintptr(syscall.MS_SHARED)
var UNSHARE uintptr = uintptr(unix.CLONE_NEWUTS | unix.CLONE_NEWPID | unix.CLONE_NEWIPC)

type Container struct {
	id         string
	rootDir    string
	codeDir    string
	scratchDir string
	cgroup     *cgroup.Cgroup
	client     *http.Client
	meta       *Meta
	cmd        *exec.Cmd
	// 1 for self, plus 1 for each child (we can't release memory
	// until all descendants are dead, because they share the
	// pages of this Container, but this is the only container
	// charged)
	cgRefCount int32
	parent     *Container
	children   map[string]*Container
}

func NewContainer(parent *Container, baseImageDir, id, rootDir, codeDir, scratchDir string, cgroup *cgroup.Cgroup, meta *Meta) (*Container, error) {
	c := &Container{
		id:         id,
		rootDir:    rootDir,
		codeDir:    codeDir,
		scratchDir: scratchDir,
		cgroup:     cgroup,
		client:     &http.Client{},
		meta:       meta,
		children:   make(map[string]*Container),
		cgRefCount: 1,
	}
	if err := c.populateRoot(baseImageDir); err != nil {
		log.Printf("failed to populate root: %v", err)
		return nil, err
	}
	if err := c.bootstrapCode(); err != nil {
		log.Printf("failed to bootstrap code: %v", err)
		return nil, err
	}
	if parent != nil {
		if err := parent.Fork(c); err != nil {
			log.Printf("failed to fork: %v", err)
			return nil, err
		}
		c.parent = parent
	} else {
		if err := c.setCommand(); err != nil {
			log.Printf("failed to set command: %v", err)
			return nil, err
		}
	}
	if err := c.StartClient(); err != nil {
		log.Printf("failed to start client: %v", err)
		return nil, err
	}
	return c, nil
}

func (c *Container) ID() string {
	return c.id
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
	sockPath := c.commsSock()
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

func (c *Container) StopClient() {
	c.client.CloseIdleConnections()
}

func (c *Container) Destroy() error {
	if err := c.cgroup.Pause(); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	return c.decCgRefCount()
}

func (c *Container) Start() error {
	c.cmd.SysProcAttr.Chroot = c.rootDir
	// c.cmd.SysProcAttr.Cloneflags = UNSHARE
	path := c.cgroup.CgroupProcsPath()
	fd, err := syscall.Open(path, syscall.O_WRONLY, 0600)
	if err != nil {
		return &ContainerError{container: c.id, err: fmt.Errorf("failed to open cgroup.procs file: %v", err)}
	}
	c.cmd.ExtraFiles = []*os.File{os.NewFile(uintptr(fd), path)}
	c.cmd.Env = []string{} // for security, DO NOT expose host env to guest
	if err := c.cmd.Start(); err != nil {
		return &ContainerError{container: c.id, err: fmt.Errorf("failed to start container: %v", err)}
	}
	return c.cmd.Wait() // Command passed in is expected to fork and exec
}

func (c *Container) Pause() error {
	if err := c.cgroup.Pause(); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	oldLimit := c.cgroup.MemLimitMB()
	newLimit := c.cgroup.GetMemUsageMB() + 1
	if newLimit < oldLimit {
		if err := c.cgroup.SetMemLimitMB(newLimit); err != nil {
			return &ContainerError{container: c.id, err: err}
		}
	}
	c.client.CloseIdleConnections()
	return nil
}

func (c *Container) Unpause() error {
	oldLimit := c.cgroup.MemLimitMB()
	newLimit := c.cgroup.GetMemUsageMB() - 1
	if newLimit > oldLimit {
		if err := c.cgroup.SetMemLimitMB(newLimit); err != nil {
			return &ContainerError{container: c.id, err: err}
		}
	}
	if err := c.cgroup.Unpause(); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	return nil
}

func (c *Container) commsSock() string {
	return fmt.Sprintf("%s/comms.sock", c.scratchDir)
}

func (c *Container) Stop() error {
	if err := c.cgroup.Pause(); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	if err := c.cgroup.KillAllProcs(); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	if err := c.cgroup.Release(); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	return nil
}

func (c *Container) Meta() *Meta {
	return c.meta
}

// fork a new process from the Zygote in container, relocate it to be the server in dst
func (c *Container) Fork(dst *Container) error {
	spareMB := c.cgroup.MemLimitMB() - c.cgroup.GetMemUsageMB()
	if spareMB < 3 {
		return fmt.Errorf("only %vMB of spare memory in parent, rejecting fork request (need at least 3MB)", spareMB)
	}

	// increment reference count before we start any processes
	c.children[dst.ID()] = dst
	newCount := atomic.AddInt32(&c.cgRefCount, 1)

	if newCount == 0 {
		return &ContainerError{container: c.id, err: fmt.Errorf("cgroup ref count went negative")}
	}

	origPids, err := c.cgroup.PIDs()
	if err != nil {
		return &ContainerError{container: c.id, err: err}
	}

	root, err := os.Open(dst.RootDir())
	if err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	defer root.Close()

	cg := dst.cgroup
	cgProcs, err := os.OpenFile(cg.CgroupProcsPath(), os.O_WRONLY, 0600)
	if err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	defer cgProcs.Close()

	err = c.forkRequest(root, cgProcs)
	if err != nil {
		return &ContainerError{container: c.id, err: err}
	}

	// move new PIDs to new cgroup.
	//
	// Make multiple passes in case new processes are being
	// spawned (TODO: better way to do this?  This lets a forking
	// process potentially kill our cache entry, which isn't
	// great).

	for {
		currPids, err := c.cgroup.PIDs()
		if err != nil {
			return &ContainerError{container: c.id, err: err}
		}

		moved := 0

		for _, pid := range currPids {
			isOrig := false
			for _, origPid := range origPids {
				if pid == origPid {
					isOrig = true
					break
				}
			}
			if !isOrig {
				c.printf("move PID %v from CG %v to CG %v\n", pid, c.cgroup.Name(), dst.cgroup.Name())
				if err = dst.cgroup.AddPid(pid); err != nil {
					return err
				}
				moved++
			}
		}

		if moved == 0 {
			break
		}
	}

	return nil
}

func (c *Container) decCgRefCount() error {
	newCount := atomic.AddInt32(&c.cgRefCount, -1)

	if newCount < 0 {
		return &ContainerError{container: c.id, err: fmt.Errorf("cgroup ref count went negative")}
	}

	if newCount == 0 {
		if c.cgroup != nil {
			if err := c.cgroup.KillAllProcs(); err != nil {
				return &ContainerError{container: c.id, err: err}
			}
			if err := c.cgroup.Release(); err != nil {
				return &ContainerError{container: c.id, err: err}
			}
		}

		if err := syscall.Unmount(c.rootDir, syscall.MNT_DETACH); err != nil {
			return &ContainerError{container: c.id, err: fmt.Errorf("failed to unmount root dir: %v", err)}
		}
		if err := os.RemoveAll(c.rootDir); err != nil {
			return &ContainerError{container: c.id, err: fmt.Errorf("failed to remove root dir: %v", err)}
		}
		if c.parent != nil {
			return c.parent.childExit(c)
		}
	}
	return nil
}

func (c *Container) childExit(child *Container) error {
	delete(c.children, child.ID())
	return c.decCgRefCount()
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

func (c *Container) setCommand() error {
	switch c.meta.Runtime {
	case Python:
		cmd := exec.Command(
			"chroot", c.rootDir, "python3", "-u",
			"/runtime/python/server.py", "/host/bootstrap.py", strconv.Itoa(1),
			strconv.FormatBool(true),
		)
		c.cmd = cmd
	case Node:
		cmd := exec.Command(
			"chroot", c.rootDir, "node",
			"/runtime/node/server.js", "/host/bootstrap.js", strconv.Itoa(1),
			strconv.FormatBool(true),
		)
		c.cmd = cmd
	case Ruby:
		cmd := exec.Command(
			"chroot", c.rootDir, "ruby",
			"/runtime/ruby/server.rb", "/host/bootstrap.rb", strconv.Itoa(1),
			strconv.FormatBool(true),
		)
		c.cmd = cmd
	default:
		return &ContainerError{container: c.id, err: fmt.Errorf("unsupported runtime: %v", c.meta.Runtime)}
	}
	return nil
}

// add ID to each log message so we know which logs correspond to
// which containers
func (c *Container) printf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("%s [SOCK %s]", strings.TrimRight(msg, "\n"), c.id)
}
