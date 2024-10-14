package build

import (
	"github.com/mandelsoft/vfs/pkg/vfs"
	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/ocm/extensions/repositories/ctf"
	"ocm.software/ocm/api/utils/misc"
	"ocm.software/ocm/api/utils/template"
)

type Options struct {
	Create    bool
	Force     bool
	ReResolve bool

	Archive   string
	Format    ctf.FormatHandler
	Mode      vfs.FileMode
	BuildFile string

	Version string

	GenDir    string
	BuildDir  string
	PluginDir string
	Printer   misc.Printer

	Templater template.Options

	Components []string
}

func (o *Options) Complete(ctx clictx.Context) error {
	if o.Version == "" {
		o.Version = "0.1.0"
	}
	if o.BuildFile == "" {
		o.BuildFile = "BuildFile.yaml"
	}
	if o.Format == nil {
		o.Format = ctf.FormatDirectory
	}
	if o.Mode == 0 {
		o.Mode = 0660
	}
	if o.Format == ctf.FormatDirectory {
		o.Mode |= 0o100
	}

	if o.GenDir == "" {
		o.GenDir = "gen"
	}
	if o.BuildDir == "" {
		o.BuildDir = o.GenDir + "/ocm"
	}
	if o.PluginDir == "" {
		o.PluginDir = o.BuildDir + "/buildplugins"
	}
	if o.Archive == "" {
		o.Archive = o.BuildDir + "/build.ctf"
	}

	if o.Printer == nil {
		o.Printer = misc.NewPrinter(ctx.StdOut())
	}

	return o.Templater.Complete(ctx.FileSystem())
}
