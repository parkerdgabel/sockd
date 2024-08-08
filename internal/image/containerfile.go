package image

import (
	"bytes"
	"parkerdgabel/sockd/pkg/container"
	"strings"
	"text/template"
)

const joinKey = "_"

type ContainerfileConfig struct {
	BaseImageName    string
	BaseImageVersion string
	Runtime          container.Runtime
}

func (c *ContainerfileConfig) Key() string {
	return strings.Join([]string{c.BaseImageName, c.BaseImageVersion, string(c.Runtime)}, joinKey)
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
