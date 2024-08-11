package image

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	strg "parkerdgabel/sockd/internal/storage"

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
)

type ImageCacheError struct {
	image string
	err   error
}

func (e *ImageCacheError) Error() string {
	return "ImageCache error: " + e.image + ": " + e.err.Error()
}

type ImageCache struct {
	imageDirs *strg.DirMaker
	images    map[string]string
}

func NewImageCache() *ImageCache {
	dirs, err := strg.NewDirMaker("images", strg.STORE_PRIVATE)
	if err != nil {
		fmt.Printf("failed to create image cache: %q", err)
		return nil
	}
	return &ImageCache{
		imageDirs: dirs,
		images:    make(map[string]string),
	}
}

func (ic *ImageCache) GetImage(name string) (string, bool) {
	image, ok := ic.images[name]
	return image, ok
}

func (ic *ImageCache) SetImage(name, path string) {
	ic.images[name] = path
}

func (ic *ImageCache) DeleteImage(name string) {
	delete(ic.images, name)
}

func (ic *ImageCache) BuildImage(config *ContainerfileConfig) error {
	if buildah.InitReexec() {
		return &ImageCacheError{config.Key(), errors.New("failed to initialize reexec")}
	}
	unshare.MaybeReexecUsingUserNamespace(false)

	buildStoreOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return &ImageCacheError{config.Key(), err}
	}

	buildStore, err := storage.GetStore(buildStoreOptions)
	if err != nil {
		return &ImageCacheError{config.BaseImageName, err}
	}
	defer func() {
		if _, err := buildStore.Shutdown(false); err != nil {
			if !errors.Is(err, storage.ErrLayerUsedByContainer) {
				fmt.Printf("failed to shutdown storage: %q", err)
			}
		}
	}()

	d, err := os.MkdirTemp("", "")
	if err != nil {
		return &ImageCacheError{config.Key(), err}
	}
	defer os.RemoveAll(d)
	containerfile := filepath.Join(d, "Containerfile")
	f, err := os.Create(containerfile)
	if err != nil {
		return &ImageCacheError{config.BaseImageName, err}
	}
	content, err := containerfileCreatorInstance.CreateContainerfile(*config)
	if err != nil {
		return &ImageCacheError{config.Key(), err}
	}
	fmt.Fprintf(f, "%s", content)
	f.Close()

	outputDir := ic.imageDirs.Make(config.Key())

	buildOptions := define.BuildOptions{
		ContextDirectory: d,
		BuildOutput:      outputDir,
	}

	_, _, err = imagebuildah.BuildDockerfiles(context.TODO(), buildStore, buildOptions, containerfile)
	if err != nil {
		return &ImageCacheError{config.Key(), err}
	}

	// PART 2: various files/dirs on top of the extracted image
	fmt.Printf("\tCreate handler/host/packages/resolve.conf over base image.\n")
	if err := os.Mkdir(path.Join(outputDir, "handler"), 0700); err != nil {
		return err
	}

	if err := os.Mkdir(path.Join(outputDir, "host"), 0700); err != nil {
		return err
	}

	if err := os.Mkdir(path.Join(outputDir, "packages"), 0700); err != nil {
		return err
	}

	// need this because Docker containers don't have a dns server in /etc/resolv.conf
	// TODO: make it a config option
	dnsPath := filepath.Join(outputDir, "etc", "resolv.conf")
	if err := ioutil.WriteFile(dnsPath, []byte("nameserver 8.8.8.8\n"), 0644); err != nil {
		return err
	}

	// PART 3: make /dev/* devices
	fmt.Printf("\tCreate /dev/(null,random,urandom) over base image.\n")
	path := filepath.Join(outputDir, "dev", "null")
	if err := exec.Command("mknod", "-m", "0644", path, "c", "1", "3").Run(); err != nil {
		return err
	}

	path = filepath.Join(outputDir, "dev", "random")
	if err := exec.Command("mknod", "-m", "0644", path, "c", "1", "8").Run(); err != nil {
		return err
	}

	path = filepath.Join(outputDir, "dev", "urandom")
	if err := exec.Command("mknod", "-m", "0644", path, "c", "1", "9").Run(); err != nil {
		return &ImageCacheError{config.Key(), err}
	}

	ic.images[config.Key()] = outputDir
	return nil
}
