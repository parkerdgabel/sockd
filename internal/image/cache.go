package image

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/containers/buildah"
	"github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
)

const joinKey = "_"

type ContainerfileConfig struct {
	BaseImageName    string
	BaseImageVersion string
	Runtime          string
}

func (c *ContainerfileConfig) Key() string {
	return strings.Join([]string{c.BaseImageName, c.BaseImageVersion, c.Runtime}, joinKey)
}

type containerfileCreator struct {
	templates *template.Template
}

func newContainerfileCreator() *containerfileCreator {
	templates := template.Must(template.ParseGlob("templates/*.tmpl"))
	return &containerfileCreator{
		templates: templates,
	}
}

func (c *containerfileCreator) CreateContainerfile(config ContainerfileConfig) ([]byte, error) {
	w := new(bytes.Buffer)
	if err := c.templates.ExecuteTemplate(w, "driver", config); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

var containerfileCreatorInstance = newContainerfileCreator()

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

	outputDir := ic.cacheDir + "/" + config.Key()

	buildOptions := define.BuildOptions{
		ContextDirectory: d,
		BuildOutput:      outputDir,
	}

	_, _, err = imagebuildah.BuildDockerfiles(context.TODO(), buildStore, buildOptions, containerfile)
	if err != nil {
		return &ImageCacheError{config.Key(), err}
	}
	ic.images[config.Key()] = outputDir
	return nil
}
