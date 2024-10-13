package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/mandelsoft/filepath/pkg/filepath"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"ocm.software/ocm/api/utils/runtime"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"
	"ocm.software/ocm/cmds/test/build/ppi"
	"ocm.software/ocm/cmds/test/build/state"
)

func main() {
	ppi.NewGenericPlugin[Config](&Handler{}, usage).Run(os.Args)
}

type Config struct {
	Cmd []json.RawMessage `json:"cmd,omitempty"`
}

type Arg struct {
	Path          string `json:"path,omitempty"`
	GoPackagePath string `json:"gopkgpath,omitempty"`
}

const usage = `
- cmd (*[]arg*) command and command arguments

arg can be a simple string or a qualified arg:
- path: <path> a patch argument relative to the build file
- gopkgpath: <path> a Go package filesystem path. If relative
  it will automatically prefixed with a ./
`

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
	var args []string
	for i, a := range cfg.Cmd {
		var simple string

		err := runtime.DefaultYAMLEncoding.Unmarshal(a, &simple)
		if err == nil {
			args = append(args, simple)
		} else {
			var arg Arg
			err := runtime.DefaultYAMLEncoding.Unmarshal(a, &arg)
			if err != nil {
				return errors.Wrapf(err, "invalid argument spec %d", i)
			}
			if arg.GoPackagePath == "" && arg.Path == "" {
				return errors.Newf("invalid argument spec %d: path or gopkgpath must be set", i)
			}
			if arg.GoPackagePath != "" && arg.Path != "" {
				return errors.Newf("invalid argument spec %d: either path or gopkgpath must be set", i)
			}
			if arg.Path != "" {
				args = append(args, p.Path(arg.Path))
			} else {
				path := p.Path(arg.GoPackagePath)
				if !filepath.IsAbs(path) {
					_, c := vfs.Components(osfs.OsFs, path)
					if c[0] != "." {
						path = strings.Join(append([]string{"."}, c...), string(os.PathSeparator))
					}
				}
				args = append(args, path)
			}
		}
	}

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	p.Printer().Printf("%s\n", strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		return errors.Wrapf(err, "execution failed")
	}
	return nil
}
