package frontend

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"text/template"
)

//go:embed templates/*
var files embed.FS
var templates map[string]*template.Template

func LoadTemplates() error {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	tmplFiles, err := fs.ReadDir(files, "templates")
	if err != nil {
		return err
	}
	for _, tmpl := range tmplFiles {
		pt, err := template.ParseFS(files, "templates/"+tmpl.Name())
		if err != nil {
			return err
		}
		templates[tmpl.Name()] = pt
	}
	return nil
}

func Render(w http.ResponseWriter, name string, data any) error {
	t, ok := templates[name+".html"]
	if !ok {
		return fmt.Errorf("template %s not found", name)
	}
	w.WriteHeader(http.StatusOK)
	if err := t.Execute(w, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}
	return nil
}
