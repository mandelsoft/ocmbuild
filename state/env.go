package state

import (
	"github.com/mandelsoft/ocm-build/utils"
)

type Environment struct {
	utils.BasePath `json:",inline"`
	GenDir         string `json:"genDir"`
}

func NewEnvironment(basedir, gendir string) *Environment {
	return &Environment{
		BasePath: utils.BasePath{
			Directory: basedir,
		},
		GenDir: gendir,
	}
}
