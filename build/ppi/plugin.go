package ppi

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

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
	handler Handler[C]
	config  C
	printer common.Printer
	env     state.Environment
}

func NewPlugin[C any](h Handler[C]) *Plugin[C] {
	return &Plugin[C]{handler: h, printer: common.StderrPrinter.AddGap("      ")}
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
	ExitOnError(p.handler.Run(p, &pstate, pstate.Components[index]), "plugin failed")

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
