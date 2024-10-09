package build

import (
	"encoding/json"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/goutils/general"
	"github.com/mandelsoft/vfs/pkg/vfs"
	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/compdesc/versions/ocm.software/v3alpha1"
	"ocm.software/ocm/api/ocm/extensions/repositories/ctf"
	"ocm.software/ocm/api/ocm/tools/transfer/transferhandler/standard"
	"ocm.software/ocm/api/utils/accessobj"
	"ocm.software/ocm/api/utils/template"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs"
	"ocm.software/ocm/cmds/test/build/state"
)

func Archive(ctx clictx.Context, opts *Options) (ocm.Repository, error) {
	fs := ctx.FileSystem()

	if ok, err := vfs.Exists(fs, opts.Archive); ok || err != nil {
		if err != nil {
			return nil, err
		}
		if opts.Force {
			err = fs.RemoveAll(opts.Archive)
			if err != nil {
				return nil, errors.Wrapf(err, "cannot remove old %q", opts.Archive)
			}
			opts.Create = true
		}
	}

	openmode := accessobj.ACC_WRITABLE
	if opts.Create {
		openmode |= accessobj.ACC_CREATE
	}
	return ctf.Open(ctx.OCMContext(), openmode, opts.Archive, opts.Mode, opts.Format, fs)
}

func ProcessDescriptions(ctx clictx.Context, h *comp.ResourceSpecHandler, opts *Options, elements ...addhdlrs.ElementSource) ([]addhdlrs.Element, inputs.Context, error) {
	var templ template.Options
	templ.Complete(ctx.FileSystem())
	return addhdlrs.ProcessDescriptions(ctx, opts.Printer, templ, h, elements)
}

func Build(ctx clictx.Context, constructor string, opts Options) error {
	return Apply(ctx, opts, common.NewElementFileSource(constructor, ctx.FileSystem()))
}

func Apply(ctx clictx.Context, opts Options, elements ...addhdlrs.ElementSource) error {
	session := ocm.NewSession(nil)
	defer session.Close()

	closure := true

	fs := ctx.FileSystem()
	err := opts.Complete(ctx)
	if err != nil {
		return err
	}
	h := comp.New(opts.Version, v3alpha1.SchemaVersion)

	elems, ictx, err := ProcessDescriptions(ctx, h, &opts, elements...)
	if err != nil {
		return err
	}

	repo, err := Archive(ctx, &opts)
	if err != nil {
		return err
	}

	thdlr, err := standard.New(standard.KeepGlobalAccess(), standard.Recursive(), standard.ResourcesByValue())
	if err != nil {
		return err
	}

	if err == nil {
		err = comp.ProcessComponents(ctx, ictx, repo, general.Conditional(closure, ctx.OCMContext().GetResolver(), nil), thdlr, h, elems)
		cerr := repo.Close()
		if err == nil {
			err = cerr
		}
	}
	if err != nil {
		if opts.Create {
			fs.RemoveAll(opts.Archive)
		}
		return err
	}

	return err
}

type ContentSource struct {
	src  addhdlrs.SourceInfo
	data []byte
}

func NewSource(path string, pstate *state.Descriptor) (addhdlrs.ElementSource, error) {
	data, err := json.Marshal(&state.Descriptor{
		Components: pstate.Components,
	})
	if err != nil {
		return nil, err
	}
	return &ContentSource{
		src:  addhdlrs.NewSourceInfo(path),
		data: data,
	}, nil
}

func (c *ContentSource) Origin() addhdlrs.SourceInfo {
	return c.src
}

func (c *ContentSource) Get() (string, error) {
	return string(c.data), nil
}

var _ addhdlrs.ElementSource = (*ContentSource)(nil)
