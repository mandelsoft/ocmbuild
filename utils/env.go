package utils

import (
	"github.com/mandelsoft/filepath/pkg/filepath"
)

type PathCompleter interface {
	Path(path string) string
}

type BasePath struct {
	Directory string `json:"directory"`
}

func (p *BasePath) Path(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(p.Directory, path)
}
