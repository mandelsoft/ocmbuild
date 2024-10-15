package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/vfs"
	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/datacontext/attrs/vfsattr"
	"ocm.software/ocm/api/utils/misc"
	"ocm.software/ocm/api/utils/runtime"

	"github.com/mandelsoft/ocm-build/buildfile"
	"github.com/mandelsoft/ocm-build/plugincache"
	"github.com/mandelsoft/ocm-build/state"
)

type Execution struct {
	ctx     clictx.Context
	opts    *Options
	plugins *plugincache.PluginCache
	fs      vfs.FileSystem

	dir       string
	buildfile *buildfile.Descriptor
	state     *state.Descriptor
}

func New(ctx clictx.Context, opts Options) (*Execution, error) {
	err := opts.Complete(ctx)
	if err != nil {
		return nil, err
	}

	plugins, err := plugincache.New(ctx.OCMContext(), opts.PluginDir, opts.Printer)
	if err != nil {
		return nil, err
	}

	fs := vfsattr.Get(ctx)
	dir := vfs.Dir(fs, opts.BuildFile)

	data, err := vfs.ReadFile(fs, opts.BuildFile)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot read build file")
	}

	d, err := opts.Templater.Templater.Process(string(data), opts.Templater.Vars)
	if err != nil {
		return nil, err
	}
	data = []byte(d)
	var bd buildfile.Descriptor

	err = runtime.DefaultYAMLEncoding.Unmarshal(data, &bd)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot decode build file")
	}

	if bd.Version == "" {
		bd.Version = opts.Version
	}

	pstate := state.New(&bd)

	execution := &Execution{
		ctx:       ctx,
		opts:      &opts,
		plugins:   plugins,
		fs:        fs,
		dir:       dir,
		buildfile: &bd,
		state:     pstate,
	}
	return execution, nil
}

func Execute(ctx clictx.Context, opts Options) error {
	e, err := New(ctx, opts)
	if err != nil {
		return err
	}
	return e.Run()
}

func (e *Execution) Run() error {
	printer := e.opts.Printer
	printer.Printf("executing build...\n")
	printer = printer.AddGap("  ")

	if len(e.buildfile.Builds) > 0 {
		printer.Printf("executing build steps...\n")
		err := e.ExecuteBuilds(printer.AddGap("  "), e.buildfile.Builds, -1, "")
		if err != nil {
			return err
		}
	}
	if len(e.buildfile.Components) > 0 {
		printer.Printf("executing component build steps...\n")
		printer := printer.AddGap("  ")
		n := 0
		for _, c := range e.buildfile.Components {
			if len(e.opts.Components) > 0 {
				found := false
				for _, t := range e.opts.Components {
					i := strings.Index(t, ":")
					if i < 0 {
						found = t == c.Name
					} else {
						found = t[:i] == c.Name && (t[i+1:] == c.Version || (c.Version == "" && t[i+1:] == e.state.BuildFile.Version))
					}
					if found {
						break
					}
				}
				if !found {
					continue
				}
			}
			res := e.state.AddComponent(&c)
			printer.Printf("building component %s...\n", misc.VersionedElementKey(res))
			err := e.ExecuteBuilds(printer.AddGap("  "), c.Builds, n, fmt.Sprintf("component %s, ", misc.VersionedElementKey(res)))
			if err != nil {
				return err
			}
			n++
		}
	}

	if len(e.state.Components) > 0 {
		elem, err := NewSource(e.opts.BuildFile, e.state)
		if err != nil {
			return err
		}

		return e.Apply(elem)
	} else {
		e.opts.Printer.Printf("no components described -> skip update transport archive\n")
		return nil
	}
}

func (e *Execution) ExecuteBuilds(printer misc.Printer, builds []buildfile.Build, n int, ectx string) error {
	for i, b := range builds {
		p, err := e.plugins.Get(&b.Plugin, e.dir)
		if err != nil {
			return errors.Wrapf(err, "%sstep %d", ectx, i+1)
		}
		hash := sha256.Sum256([]byte(fmt.Sprintf("%s%s::%d", ectx, p.Path(), i)))
		gendir := vfs.Join(e.fs, e.opts.BuildDir, "steps", hex.EncodeToString(hash[:]))
		env := state.NewEnvironment(e.dir, gendir)
		printer.Printf("step %d[%s] in %s...\n", i+1, p.String(), gendir)

		nstate, err := e.ExecutePlugin(p, n, b.Config, env)
		if err != nil {
			return errors.Wrapf(err, "%sstep %d", ectx, i+1)
		}
		e.state = nstate
	}
	return nil
}

func (e *Execution) ExecutePlugin(p *plugincache.Plugin, index int, config json.RawMessage, env *state.Environment) (*state.Descriptor, error) {
	envdata, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(p.Path(), p.Args(string(envdata), strconv.Itoa(index), string(config))...)

	data, err := json.Marshal(e.state)
	if err != nil {
		return nil, err
	}

	out := bytes.NewBuffer(nil)

	cmd.Stdin = bytes.NewBuffer(data)
	cmd.Stdout = out
	cmd.Stderr = e.ctx.StdOut()

	err = cmd.Run()
	if err != nil {
		return nil, err
	}

	var result state.Descriptor
	err = json.Unmarshal(out.Bytes(), &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}
