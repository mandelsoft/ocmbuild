package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/ocm-build/ppi"
	utils2 "github.com/mandelsoft/ocm-build/utils"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"

	"github.com/mandelsoft/ocm-build/state"
)

func main() {
	ppi.NewGenericPlugin[Config](&Handler{}, usage).Run(os.Args)
}

type Config struct {
	Cmd json.RawMessage `json:"cmd,omitempty"`
}

const usage = `
- cmd (*[]arg*) command and command arguments

arg can be a simple string or a qualified arg:
- path: <path> a patch argument relative to the build file
- gopkgpath: <path> a Go package filesystem path. If relative
  it will automatically prefixed with a ./
`

type Arg struct {
	Path          string `json:"path,omitempty"`
	GoPackagePath string `json:"gopkgpath,omitempty"`
}

type Handler struct{}

var _ ppi.Handler[Config] = (*Handler)(nil)

func (h *Handler) Run(p *ppi.Plugin[Config], _ *state.Descriptor, _ *comp.ResourceSpec) error {
	config := p.Config()

	if len(config.Cmd) == 0 {
		return fmt.Errorf("at least a command name is required")
	}

	return build(p, config)
}

func build(p *ppi.Plugin[Config], cfg *Config) error {
	args, err := utils2.Args(p, cfg.Cmd)
	if err != nil {
		return err
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	p.Printer().Printf("%s\n", strings.Join(args, " "))
	err = cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "execution failed")
	}
	return nil
}
