package zygote

import (
	"parkerdgabel/sockd/internal/storage"
	"parkerdgabel/sockd/pkg/cgroup"
	"parkerdgabel/sockd/pkg/container"
)

type Provider interface {
	ProvideZygote(codeDir string, meta *container.Meta) (*container.Container, error)
	MemPool() *MemPool
}

type importCacheProvider struct {
	ic  *importCache
	mem *MemPool
}

func (icp *importCacheProvider) MemPool() *MemPool {
	return icp.mem
}

func NewImportCacheProvider(ic *importCache) Provider {
	// TODO: Configure the MemPool
	mem := NewMemPool("zygote", 100)
	return &importCacheProvider{ic: ic, mem: mem}
}

func (icp *importCacheProvider) ProvideZygote(codeDir string, meta *container.Meta) (*container.Container, error) {
	return icp.ic.Create(meta)
}

func NewProvider(rootDirs, codeDirs, scratchDirs *storage.DirMaker, baseImageDir string, cgroupPool *cgroup.Pool, pullerInstaller container.PackagePullerInstaller) Provider {
	return NewImportCacheProvider(newImportCache(rootDirs, codeDirs, scratchDirs, baseImageDir, cgroupPool, pullerInstaller))
}
