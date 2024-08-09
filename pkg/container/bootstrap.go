package container

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"text/template"
)

var ErrEmptyBootstrap = errors.New("empty bootstrap code")

type bootstrapper struct {
	templates *template.Template
}

func (b *bootstrapper) bootstrapCode(c *Container) ([]byte, error) {
	w := bytes.NewBuffer(nil)
	if err := b.templates.ExecuteTemplate(w, "driver", c); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func newBootstrapper() *bootstrapper {
	pattern := filepath.Join("templates", "*.tmpl")
	templates := template.Must(template.ParseGlob(pattern))
	return &bootstrapper{templates: templates}
}

var b = newBootstrapper()

func (c *Container) bootstrapCode() error {
	code, err := b.bootstrapCode(c)
	if err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	if len(code) == 0 {
		return &ContainerError{container: c.id, err: ErrEmptyBootstrap}
	}
	var path string
	switch c.meta.Runtime {
	case Python:
		path = filepath.Join(c.scratchDir, "bootstrap.py")
	case Node:
		path = filepath.Join(c.scratchDir, "bootstrap.js")
	case Ruby:
		path = filepath.Join(c.scratchDir, "bootstrap.rb")
	default:
		panic("Not implemented")
	}
	if err := os.WriteFile(path, code, 0600); err != nil {
		return &ContainerError{container: c.id, err: err}
	}
	return nil
}
