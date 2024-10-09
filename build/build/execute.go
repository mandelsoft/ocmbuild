package build

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/vfs"
	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/datacontext/attrs/vfsattr"
	"ocm.software/ocm/api/utils/misc"
	"ocm.software/ocm/api/utils/runtime"
	"ocm.software/ocm/cmds/test/build/buildfile"
	"ocm.software/ocm/cmds/test/build/plugin"
	"ocm.software/ocm/cmds/test/build/state"
)

func Execute(ctx clictx.Context, opts Options) error {
	err := opts.Complete(ctx)
	if err != nil {
		return err
	}

	plugins, err := plugin.New(ctx.OCMContext(), opts.PluginDir, opts.Printer)
	if err != nil {
		return err
	}

	fs := vfsattr.Get(ctx)
	dir := vfs.Dir(fs, opts.BuildFile)

	data, err := vfs.ReadFile(fs, opts.BuildFile)
	if err != nil {
		return errors.Wrapf(err, "cannot read build file")
	}

	d, err := opts.Templater.Templater.Process(string(data), opts.Templater.Vars)
	if err != nil {
		return err
	}
	data = []byte(d)
	var buildfile buildfile.Descriptor

	err = runtime.DefaultYAMLEncoding.Unmarshal(data, &buildfile)
	if err != nil {
		return errors.Wrapf(err, "cannot decode build file")
	}

	pstate := state.New(&buildfile)

	opts.Printer.Printf("executing build....\n")
	for n, c := range buildfile.Components {
		res := pstate.AddComponent(&c)
		opts.Printer.Printf("  building component %s...\n", misc.VersionedElementKey(res))
		for i, b := range c.Builds {
			p, err := plugins.Get(&b.Plugin, dir)
			if err != nil {
				return errors.Wrapf(err, "component %s, plugin %d", misc.VersionedElementKey(res), i)
			}
			hash := sha256.Sum256([]byte(fmt.Sprintf("%s::%d", p.Path(), i)))
			gendir := vfs.Join(fs, opts.BuildDir, "steps", hex.EncodeToString(hash[:]))
			env := &state.Environment{
				Directory: dir,
				GenDir:    gendir,
			}
			opts.Printer.Printf("    step %d[%s] in %s...\n", i, p.String(), gendir)

			nstate, err := ExecutePlugin(ctx, p.Path(), n, b.Config, pstate, env)
			if err != nil {
				return errors.Wrapf(err, "component %s, step %d", misc.VersionedElementKey(res), i)
			}
			pstate = nstate
		}
	}

	elem, err := NewSource(opts.BuildFile, pstate)
	if err != nil {
		return err
	}

	return Apply(ctx, opts, elem)
}

func ExecutePlugin(ctx clictx.Context, p string, index int, config json.RawMessage, pstate *state.Descriptor, env *state.Environment) (*state.Descriptor, error) {
	envdata, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(p, string(envdata), strconv.Itoa(index), string(config))

	data, err := json.Marshal(pstate)
	if err != nil {
		return nil, err
	}

	out := bytes.NewBuffer(nil)

	cmd.Stdin = bytes.NewBuffer(data)
	cmd.Stdout = out
	cmd.Stderr = ctx.StdOut()

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
