package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"
	"ocm.software/ocm/cmds/test/build/ppi"
	"ocm.software/ocm/cmds/test/build/state"
)

func main() {
	ppi.NewPlugin[Config](&Handler{}).Run(os.Args)
}

type Config struct {
	Path         string   `json:"path"`
	SourceOption string   `json:"sourceOption,omitempty"`
	SourcePath   string   `json:"sourcePath,omitempty"`
	Args         []string `json:"args,omitempty"`
}

type Handler struct{}

var _ ppi.Handler[Config] = (*Handler)(nil)

func (h *Handler) Run(p *ppi.Plugin[Config], pstate *state.Descriptor, c *comp.ResourceSpec) error {
	config := p.Config()

	if config.Path == "" {
		return fmt.Errorf("file path to build required")
	}

	return build(p, config)
}

func build(p *ppi.Plugin[Config], cfg *Config) error {
	args := cfg.Args
	if cfg.SourcePath != "" {
		args = nil
		if cfg.SourceOption != "" {
			args = []string{cfg.SourceOption}
		}
		args = append(append(args, p.Path(cfg.SourcePath)), cfg.Args...)
	}

	path := p.Path(cfg.Path)
	if ok, err := vfs.Exists(osfs.OsFs, path); !ok || err != nil {
		return fmt.Errorf("path %q not found", path)
	}

	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	p.Printer().Printf("%s\n", path, strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "execution failed")
	}
	return nil
}
