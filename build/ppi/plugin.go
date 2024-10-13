package ppi

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	common "ocm.software/ocm/api/utils/misc"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"
	"ocm.software/ocm/cmds/test/build/state"
)

type Handler[C any] interface {
	Run(p *Plugin[C], pstate *state.Descriptor, c *comp.ResourceSpec) error
}

type Plugin[C any] struct {
	comp    bool
	handler Handler[C]
	config  C
	printer common.Printer
	env     state.Environment
	usage   string
}

func NewPlugin[C any](h Handler[C], usage ...string) *Plugin[C] {
	return &Plugin[C]{comp: true, handler: h, printer: common.StderrPrinter.AddGap("      "), usage: strings.Join(usage, "\n")}
}

func NewGenericPlugin[C any](h Handler[C], usage ...string) *Plugin[C] {
	return &Plugin[C]{comp: false, handler: h, printer: common.StderrPrinter.AddGap("      "), usage: strings.Join(usage, "\n")}
}

func (p *Plugin[C]) Printer() common.Printer {
	return p.printer
}

func (p *Plugin[C]) Config() *C {
	return &p.config
}

func (p *Plugin[C]) Path(path string) string {
	if vfs.IsAbs(osfs.OsFs, path) {
		return path
	}
	return vfs.Join(osfs.OsFs, p.env.Directory, path)
}

func (p *Plugin[C]) GenDir(path string) string {
	if vfs.IsAbs(osfs.OsFs, path) {
		return path
	}
	return vfs.Join(osfs.OsFs, p.env.GenDir, path)
}

func (p *Plugin[C]) Run(args []string) {
	if len(args) > 1 && args[1] == "--help" {
		ctx := `This build plugin is usable for both, sole build steps and component build
steps.`
		if p.comp {
			ctx = `This build plugin is usable for component build steps, only.`
		}
		fmt.Fprintf(os.Stderr, `Usage: %s <env json> <index> <config>\n
Stdin is used to pass the processing state, if index > 0. The index is the
index of the component version entry in the component component constructor
list. The modified state is taken from stdout. Stderr can be used to provide
regular text output and error messages.
%s
The config is taken from the plugin config in the Buildfile and uses the
following fields:
%s
`, args[0], ctx, p.usage)
	}
	if len(args) != 4 {
		Error("usage: %s <env> <index> <config> (found %#v)", args[0], args)
	}

	err := json.Unmarshal([]byte(args[1]), &p.env)
	ExitOnError(err, "invalid environment spec")

	index, err := strconv.ParseInt(args[2], 10, 32)
	ExitOnError(err, "invalid index")

	err = json.Unmarshal([]byte(args[3]), &p.config)
	ExitOnError(err, "cannot parse config")

	data, err := io.ReadAll(os.Stdin)
	ExitOnError(err, "cannot read state")

	var pstate state.Descriptor
	err = json.Unmarshal(data, &pstate)
	ExitOnError(err, "cannot unmarshal state")

	if len(pstate.Components) <= int(index) {
		Error("index %d out of range", index)
	}
	if index < 0 {
		if p.comp {
			Error("plugin suitable to component build steps, only")
		}
		ExitOnError(p.handler.Run(p, &pstate, nil), "plugin failed")
	} else {
		ExitOnError(p.handler.Run(p, &pstate, pstate.Components[index]), "plugin failed")
	}

	data, err = json.Marshal(pstate)
	ExitOnError(err, "cannot marshal state")
	_, err = os.Stdout.Write(data)
	ExitOnError(err, "cannot write output")
}

func Error(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}

func ExitOnError(err error, msg string, args ...interface{}) {
	if err != nil {
		Error("%s: %s", fmt.Sprintf(msg, args...), err.Error())
	}
}
