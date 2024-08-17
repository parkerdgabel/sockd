package zygote

import (
	"errors"
	"log"
	"parkerdgabel/sockd/internal/storage"
	"parkerdgabel/sockd/pkg/cgroup"
	"parkerdgabel/sockd/pkg/container"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

var ErrNoZygoteFound = errors.New("no Zygote found")

type importCache struct {
	rootDirs        *storage.DirMaker
	codeDirs        *storage.DirMaker
	scratchDirs     *storage.DirMaker
	baseImageDir    string
	root            *importCacheNode
	cgroupPool      *cgroup.Pool
	pullerInstaller container.PackagePullerInstaller
}

func newImportCache(rootDirs, codeDirs, scratchDirs *storage.DirMaker, baseImageDir string, cgroupPool *cgroup.Pool, pullerInstaller container.PackagePullerInstaller) *importCache {
	return &importCache{
		rootDirs:        rootDirs,
		codeDirs:        codeDirs,
		scratchDirs:     scratchDirs,
		baseImageDir:    baseImageDir,
		root:            &importCacheNode{},
		cgroupPool:      cgroupPool,
		pullerInstaller: pullerInstaller,
	}
}

type importCacheNode struct {
	packages         []string
	children         []*importCacheNode
	parent           *importCacheNode
	indirectPackages []string

	mutex      sync.Mutex
	container  *container.Container
	sbRefCount int

	createNonleafChild int64
	createLeafChild    int64

	// Sandbox for this node of the tree (may be nil); codeDir
	// doesn't contain a lambda, but does contain a packages dir
	// linking to the packages in Packages and indirectPackages.
	// Lazily initialized when Sandbox is first needed.
	codeDir string

	// inferred from Packages (lazily initialized when Sandbox is
	// first needed)
	meta *container.Meta
}

func (ic *importCache) Create(meta *container.Meta) (*container.Container, error) {
	node := ic.root.Lookup(meta.Installs)
	if node == nil {
		return nil, ErrNoZygoteFound
	}
	log.Panicf("Creating contronaer from zygote %v", node)
	return ic.createChildContainerFromNode(node)
}

func (icn *importCacheNode) Create(meta *container.Meta, baseImageDir string, codeDir string, scrachDir string, cgroupPool *cgroup.Pool) (*container.Container, error) {
	return nil, nil
}

func (ic *importCache) getContainerInNode(node *importCacheNode, forceNew bool) (*container.Container, bool, error) {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	if forceNew && node.container != nil {
		old := node.container
		node.container = nil
		go old.Destroy()
	}

	if node.container != nil {
		// FAST PATH
		if node.sbRefCount == 0 {
			if err := node.container.Unpause(); err != nil {
				node.container = nil
				return nil, false, err
			}
		}
		node.sbRefCount += 1
		return node.container, false, nil
	}

	// SLOW PATH
	if err := ic.createContainerInNode(node); err != nil {
		return nil, false, err
	}
	node.sbRefCount = 1

	return node.container, true, nil
}

func (ic *importCache) createContainerInNode(node *importCacheNode) error {
	// populate codeDir/packages with deps, and record top-level mods)
	if node.codeDir == "" {
		codeDir := ic.codeDirs.Make("import-cache")
		// TODO: clean this up upon failure

		installs, err := ic.pullerInstaller.InstallPackages(node.packages)
		if err != nil {
			return err
		}

		topLevelMods := []string{}
		for _, name := range node.packages {
			pkg, err := ic.pullerInstaller.PullPackage(name)
			if err != nil {
				return err
			}
			topLevelMods = append(topLevelMods, pkg.Meta.TopLevel...)
		}

		node.codeDir = codeDir

		// policy: what modules should we pre-import?  Top-level of
		// pre-initialized packages is just one possibility...
		node.meta = &container.Meta{
			Installs: installs,
			Imports:  topLevelMods,
		}
	}

	var c *container.Container
	if node.parent != nil {
		c, err := ic.createChildContainerFromNode(node)
		if err != nil {
			return err
		}
		if c != nil {
			node.container = c
			return nil
		}
	} else {
		node.meta = node.meta.MakeZygote()
		id := uuid.NewString()
		rootDir := ic.rootDirs.Make("import-cache-" + id)
		scratchDir := ic.scratchDirs.Make("import-cache")
		cgroup, err := ic.cgroupPool.RetrieveCgroup(time.Duration(1) * time.Second)
		if err != nil {
			return err
		}
		c, err = container.NewContainer(nil, ic.baseImageDir, id, rootDir, node.codeDir, scratchDir, cgroup, node.meta)
		if err != nil {
			return err
		}
	}

	node.container = c
	return nil
}

func (ic *importCache) createChildContainerFromNode(node *importCacheNode) (*container.Container, error) {
	// try twice, restarting parent Sandbox if it fails the first time
	forceNew := false
	for i := 0; i < 2; i++ {
		zygote, isNew, err := ic.getContainerInNode(node.parent, forceNew)
		if err != nil {
			return nil, err
		}
		id := uuid.NewString()
		rootDir := ic.rootDirs.Make("import-cache-" + id)
		scratchDir := ic.scratchDirs.Make("import-cache")
		cgroup, err := ic.cgroupPool.RetrieveCgroup(time.Duration(1) * time.Second)
		if err != nil {
			return nil, err
		}
		c, err := container.NewContainer(zygote, ic.baseImageDir, id, rootDir, node.codeDir, scratchDir, cgroup, node.meta)
		if err == nil {
			if !node.meta.IsZygote() {
				atomic.AddInt64(&node.createLeafChild, 1)
			} else {
				atomic.AddInt64(&node.createNonleafChild, 1)
			}
		}

		ic.putContainerInNode(node, zygote)
		if isNew || err != nil {
			return c, err
		}
		forceNew = true
	}
	return nil, nil
}

func (ic *importCache) putContainerInNode(node *importCacheNode, c *container.Container) {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	if node.container != c {
		return
	}

	node.sbRefCount -= 1

	if node.sbRefCount == 0 {
		if err := c.Pause(); err != nil {
			node.container = nil
		}
	}

	if node.sbRefCount == 0 {
		panic("sbRefCount should never be negative")
	}
}

func (icn *importCacheNode) Lookup(pkgs []string) *importCacheNode {
	// if this node imports a package that's not wanted by the
	// lambda, neither this Zygote nor its children will work
	for _, nodePkg := range icn.packages {
		found := false
		for _, p := range pkgs {
			if p == nodePkg {
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	// check our descendents; is one of them a Zygote that works?
	// we prefer a child Zygote over the one for this node,
	// because they have more packages pre-imported
	for _, child := range icn.children {
		result := child.Lookup(pkgs)
		if result != nil {
			return result
		}
	}

	return icn
}
