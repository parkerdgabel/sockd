package image

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

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
	cacheDir string
	images   map[string]string
}

func NewImageCache() *ImageCache {
	cacheDir := os.TempDir()
	return &ImageCache{
		cacheDir: cacheDir,
		images:   make(map[string]string),
	}
}

func (ic *ImageCache) CacheDir() string {
	return ic.cacheDir
}

func (ic *ImageCache) Close() {
	os.RemoveAll(ic.cacheDir)
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

func (ic *ImageCache) BuildImage(name string) error {
	if buildah.InitReexec() {
		return &ImageCacheError{name, errors.New("failed to initialize reexec")}
	}
	unshare.MaybeReexecUsingUserNamespace(false)

	buildStoreOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return &ImageCacheError{name, err}
	}

	buildStore, err := storage.GetStore(buildStoreOptions)
	if err != nil {
		return &ImageCacheError{name, err}
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
		return &ImageCacheError{name, err}
	}
	defer os.RemoveAll(d)
	dockerfile := filepath.Join(d, "Dockerfile")
	f, err := os.Create(dockerfile)
	if err != nil {
		return &ImageCacheError{name, err}
	}
	fmt.Fprintf(f, "FROM %s\n", name)
	f.Close()

	outputDir := ic.cacheDir + "/" + name

	buildOptions := define.BuildOptions{
		ContextDirectory: d,
		BuildOutput:      outputDir,
	}

	_, _, err = imagebuildah.BuildDockerfiles(context.TODO(), buildStore, buildOptions, dockerfile)
	if err != nil {
		return &ImageCacheError{name, err}
	}
	ic.images[name] = outputDir
	return nil
}
