package container

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"parkerdgabel/sockd/pkg/cgroup"
	"parkerdgabel/sockd/pkg/container/embedded"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

var ErrUnsupportedRuntime = errors.New("unsupported runtime")

type PullPackageRequest struct {
	Pkg              string `json:"Pkg"`
	AlreadyInstalled bool   `json:"AlreadyInstalled"`
}

type Package struct {
	Name         string
	Meta         PackageMeta
	installMutex sync.Mutex
	installed    uint32
}

// the pip-install admin lambda returns this
type PackageMeta struct {
	Deps     []string `json:"Deps"`
	TopLevel []string `json:"TopLevel"`
}

type PackagePuller interface {
	PullPackage(pkg string) (*Package, error)
}

type PackageInstaller interface {
	InstallPackages(pkgs []string) ([]string, error)
}

type PackagePullerInstaller interface {
	PackagePuller
	PackageInstaller
}

func NewPackagePullerInstaller(meta *Meta, baseImageDir string, rootDir string, cgroup *cgroup.Cgroup) (PackagePullerInstaller, error) {
	switch meta.Runtime {
	case Python:
		m := &Meta{
			Runtime:  Python,
			isLeaf:   true,
			ParentID: "",
		}
		pipLambdaDir := filepath.Join(baseImageDir, "admin-lambdas", "pip-lambda")
		packageDir := filepath.Join(baseImageDir, "packages")
		code := embedded.PyPiPullerInstaller_py
		if err := os.WriteFile(filepath.Join(pipLambdaDir, "f.py"), []byte(code), 0700); err != nil {
			return nil, err
		}
		return &PyPiPullerInstaller{
			packages:      sync.Map{},
			rootDir:       rootDir,
			baseImageDir:  baseImageDir,
			pipLambdaDir:  pipLambdaDir,
			containerMeta: m,
			packageDir:    packageDir,
			cgroup:        cgroup,
		}, nil
	default:
		return nil, ErrUnsupportedRuntime
	}

}

type PyPiPullerInstaller struct {
	packages sync.Map
	// directory of lambda code that installs pip packages
	pipLambdaDir  string
	containerMeta *Meta
	rootDir       string
	baseImageDir  string
	packageDir    string
	cgroup        *cgroup.Cgroup
}

func (p *PyPiPullerInstaller) InstallPackages(pkgs []string) ([]string, error) {
	return nil, nil
}

func (p *PyPiPullerInstaller) NormalizePackage(pkg string) string {
	return strings.ReplaceAll(strings.ToLower(pkg), "_", "-")
}

func (p *PyPiPullerInstaller) PullPackage(pkg string) (*Package, error) {
	pkg = p.NormalizePackage(pkg)
	tmp, _ := p.packages.LoadOrStore(pkg, &Package{Name: pkg})
	pa := tmp.(*Package)

	// fast path
	if atomic.LoadUint32(&pa.installed) == 1 {
		return pa, nil
	}

	pa.installMutex.Lock()
	defer pa.installMutex.Unlock()
	if pa.installed == 0 {
		if err := p.sandboxInstall(pa); err != nil {
			return pa, err
		}

		atomic.StoreUint32(&pa.installed, 1)

		return pa, nil
	}

	return pa, nil
}

func (p *PyPiPullerInstaller) sandboxInstall(pa *Package) error {
	// install the package
	scratchDir := filepath.Join(p.packageDir, pa.Name)
	log.Printf("pip install using scratchDir='%v'", scratchDir)
	alreadyInstalled := false
	if _, err := os.Stat(scratchDir); err == nil {
		log.Printf("Package %v already installed", pa.Name)
		alreadyInstalled = true
	} else {
		log.Printf("run pip install %s from a new Sandbox to %s on host", pa.Name, scratchDir)
		if err := os.Mkdir(scratchDir, 0700); err != nil {
			return err
		}
	}
	var err error
	defer func() {
		if err != nil {
			os.RemoveAll(scratchDir)
		}
	}()

	defer p.cgroup.Release()

	if err != nil {
		return err
	}

	container, err := NewContainer(nil, p.baseImageDir, uuid.New().String(), p.rootDir, p.pipLambdaDir, scratchDir, p.cgroup, p.containerMeta, nil)
	if err != nil {
		return err
	}

	if err := container.Start(); err != nil {
		return err
	}
	defer container.Destroy()

	pkgReq := PullPackageRequest{
		Pkg:              pa.Name,
		AlreadyInstalled: alreadyInstalled,
	}

	pkgReqBytes, err := json.Marshal(pkgReq)
	if err != nil {
		return err
	}
	// Host name is irrelevant as it is a local socket connection
	req, err := http.NewRequest("POST", "http://lambda/run/pip-lambda", bytes.NewBuffer(pkgReqBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := container.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		log.Printf("Failed to install package %s: %v", pa.Name, res.Status)
		return err
	}

	if err := json.NewDecoder(res.Body).Decode(&pa.Meta); err != nil {
		return err
	}

	for i, pkg := range pa.Meta.Deps {
		pa.Meta.Deps[i] = p.NormalizePackage(pkg)
	}

	return nil
}
