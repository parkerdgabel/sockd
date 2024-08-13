package bootstrap

import (
	"bytes"
	"embed"
	"errors"
	"text/template"
)

var ErrEmptyBootstrap = errors.New("empty bootstrap code")

type data struct {
	IsLeaf   bool
	Installs []string
	Imports  []string
	Runtime  string
}

type bootstrapper struct {
	templates *template.Template
}

func (b *bootstrapper) bootstrapCode(isLeaf bool, install []string, imports []string, runtime string) ([]byte, error) {
	w := bytes.NewBuffer(nil)
	d := &data{
		IsLeaf:   isLeaf,
		Installs: install,
		Imports:  imports,
		Runtime:  runtime,
	}
	if err := b.templates.ExecuteTemplate(w, "driver", d); err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

//go:embed templates/*
var templates embed.FS

func newBootstrapper() *bootstrapper {
	t := template.Must(template.ParseFS(templates, "templates/*.tmpl"))
	return &bootstrapper{templates: t}
}

var b = newBootstrapper()

func BootstrapCode(isLeaf bool, install []string, imports []string, runtime string) ([]byte, error) {
	return b.bootstrapCode(isLeaf, install, imports, runtime)
}
