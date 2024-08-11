package image

import (
	"bytes"
	"embed"
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

//go:embed templates/*
var templates embed.FS

func newContainerfileCreator() *containerfileCreator {
	t := template.Must(template.ParseFS(templates, "templates/*.tmpl"))
	return &containerfileCreator{
		templates: t,
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
