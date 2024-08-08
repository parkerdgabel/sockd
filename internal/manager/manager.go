package manager

import (
	"fmt"
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
	return &Manager{
		rootDirs:    rootDirs,
		scratchDirs: scratchDirs,
		codeDirs:    codeDirs,
		cgroupPool:  pool,
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

func (m *Manager) CreateContainer(baseImageName string, baseImageVersion string, meta *container.Meta, name string) (*container.Container, error) {
	config := &image.ContainerfileConfig{
		BaseImageName:    baseImageName,
		BaseImageVersion: baseImageVersion,
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
	container, err := container.NewContainer(dir, id, rootDir, codeDir, scratchDir, cgroup, meta)
	if err != nil {
		return nil, err
	}

	m.SetContainer(id, container)
	return container, nil
}

func (m *Manager) StartContainer(id string) error {
	container, ok := m.GetContainer(id)
	if !ok {
		return fmt.Errorf("container not found")
	}
	return container.Start()
}
