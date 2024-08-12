package zygote

import (
	"parkerdgabel/sockd/internal/storage"
	"parkerdgabel/sockd/pkg/cgroup"
	"parkerdgabel/sockd/pkg/container"
)

type Provider interface {
	ProvideZygote(codeDir string, meta *container.Meta) (*container.Container, error)
}

type importCacheProvider struct {
	ic *importCache
}

func NewImportCacheProvider(ic *importCache) Provider {
	return &importCacheProvider{ic: ic}
}

func (icp *importCacheProvider) ProvideZygote(codeDir string, meta *container.Meta) (*container.Container, error) {
	return icp.ic.Create(meta)
}

func NewProvider(rootDirs, codeDirs, scratchDirs *storage.DirMaker, baseImageDir string, cgroupPool *cgroup.Pool, pullerInstaller container.PackagePullerInstaller) Provider {
	return NewImportCacheProvider(newImportCache(rootDirs, codeDirs, scratchDirs, baseImageDir, cgroupPool, pullerInstaller))
}
