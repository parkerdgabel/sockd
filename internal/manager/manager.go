package manager

import (
	"fmt"
	"os"
	"parkerdgabel/sockd/internal/image"
	"parkerdgabel/sockd/internal/storage"
	"parkerdgabel/sockd/pkg/cgroup"
	"parkerdgabel/sockd/pkg/container"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Manager struct {
	rootDirs    *storage.DirMaker
	scratchDirs *storage.DirMaker
	codeDirs    *storage.DirMaker
	imageCache  *image.ImageCache
	cgroupPool  *cgroup.Pool
	ppPool      *cgroup.Pool
	mapMutex    sync.Mutex
	containers  map[string]*container.Container
}

func NewManager() *Manager {
	rootDirs, err := storage.NewDirMaker("root", storage.STORE_PRIVATE)
	if err != nil {
		return nil
	}
	scratchDirs, err := storage.NewDirMaker("scratch", storage.STORE_PRIVATE)
	if err != nil {
		return nil
	}

	codeDirs, err := storage.NewDirMaker("code", storage.STORE_PRIVATE)
	if err != nil {
		return nil
	}

	pool, err := cgroup.NewPool("sockd")
	if err != nil {
		return nil
	}
	ppPool, err := cgroup.NewPool("sockd_pp")
	if err != nil {
		return nil
	}
	return &Manager{
		rootDirs:    rootDirs,
		scratchDirs: scratchDirs,
		codeDirs:    codeDirs,
		cgroupPool:  pool,
		ppPool:      ppPool,
		imageCache:  image.NewImageCache(),
		containers:  make(map[string]*container.Container),
		mapMutex:    sync.Mutex{},
	}
}

func (m *Manager) GetContainer(id string) (*container.Container, bool) {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	container, ok := m.containers[id]
	return container, ok
}

func (m *Manager) SetContainer(name string, container *container.Container) {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	m.containers[name] = container
}

func (m *Manager) CreateContainer(meta *container.Meta, name string) (*container.Container, error) {
	config := &image.ContainerfileConfig{
		BaseImageName:    meta.BaseImageName,
		BaseImageVersion: meta.BaseImageVersion,
		Runtime:          meta.Runtime,
	}
	dir, found := m.imageCache.GetImage(config.Key())
	if !found {
		if err := m.imageCache.BuildImage(config); err != nil {
			return nil, err
		}
		dir, found = m.imageCache.GetImage(config.Key())
		if !found {
			return nil, fmt.Errorf("failed to build image")
		}
	}
	id := uuid.New().String()
	rootDir := m.rootDirs.Make(id)
	scratchDir := m.scratchDirs.Make(id)
	codeDir := m.codeDirs.Make(id)
	cgroup, err := m.cgroupPool.RetrieveCgroup(time.Duration(1) * time.Second)
	if err != nil {
		return nil, err
	}
	parent, ok := m.GetContainer(meta.ParentID)
	if !ok {
		return nil, fmt.Errorf("parent container not found")
	}

	if err := m.installPackages(meta, dir); err != nil {
		return nil, err
	}

	container, err := container.NewContainer(parent, dir, id, rootDir, codeDir, scratchDir, cgroup, meta)
	if err != nil {
		return nil, err
	}

	m.SetContainer(id, container)
	return container, nil
}

func (m *Manager) installPackages(meta *container.Meta, baseImageDir string) error {
	for _, pkg := range meta.Installs {
		ppRootDir := m.rootDirs.Make("pp-" + pkg)
		cgroup, err := m.ppPool.RetrieveCgroup(time.Duration(1) * time.Second)
		defer cgroup.Release()
		if err != nil {
			return err
		}
		puller, err := container.NewPackagePuller(meta, baseImageDir, ppRootDir, cgroup)
		if err != nil {
			return err
		}
		if _, err := puller.PullPackage(pkg); err != nil {
			return err
		}
		if err := os.RemoveAll(ppRootDir); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) StartContainer(id string) error {
	container, ok := m.GetContainer(id)
	if !ok {
		return fmt.Errorf("container not found")
	}
	return container.Start()
}

func (m *Manager) ListContainers() []string {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	containers := make([]string, 0, len(m.containers))
	for k := range m.containers {
		containers = append(containers, k)
	}
	return containers
}

func (m *Manager) DestroyContainer(id string) error {
	container, ok := m.GetContainer(id)
	if !ok {
		return fmt.Errorf("container not found")
	}
	if err := container.Destroy(); err != nil {
		return err
	}
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	delete(m.containers, id)
	return nil
}

func (m *Manager) ForkContainer(id string) error {
	container, ok := m.GetContainer(id)
	if !ok {
		return fmt.Errorf("container not found")
	}
	dstContainer, err := m.CreateContainer(container.Meta(), "forked")
	if err != nil {
		return err
	}
	if err := container.Fork(dstContainer); err != nil {
		return err
	}

	return nil
}

func (m *Manager) StopContainer(id string) error {
	container, ok := m.GetContainer(id)
	if !ok {
		return fmt.Errorf("container not found")
	}
	return container.Stop()
}

func (m *Manager) PauseContainer(id string) error {
	container, ok := m.GetContainer(id)
	if !ok {
		return fmt.Errorf("container not found")
	}
	return container.Pause()
}

func (m *Manager) UnpauseContainer(id string) error {
	container, ok := m.GetContainer(id)
	if !ok {
		return fmt.Errorf("container not found")
	}
	return container.Unpause()
}

func (m *Manager) Shutdown() error {
	m.mapMutex.Lock()
	defer m.mapMutex.Unlock()
	for _, container := range m.containers {
		container.Destroy()
	}
	if err := m.cgroupPool.Destroy(); err != nil {
		return err
	}

	if err := m.ppPool.Destroy(); err != nil {
		return err
	}

	return nil
}
