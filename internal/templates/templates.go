package templates

import (
	"html/template"
	"path/filepath"
)

func Load() (*template.Template, error) {
	return template.ParseGlob(filepath.Join("templates", "*.gohtml"))
}
