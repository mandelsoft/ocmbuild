package build

import (
	"os"

	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	clictx "ocm.software/ocm/api/cli"
)

func Clean(ctx clictx.Context, opts Options, builds, cache bool) error {
	e, err := New(ctx, opts)
	if err != nil {
		return err
	}
	return e.Clean(builds, cache)
}

func (e *Execution) Clean(builds, cache bool) error {
	var err error
	if cache {
		e.opts.Printer.Printf("cleaning plugin cache %s...\n", e.opts.PluginDir)
		if ok, err := vfs.DirExists(osfs.OsFs, e.opts.PluginDir); ok && err == nil {
			err = os.RemoveAll(e.opts.PluginDir)
		}
	}
	if builds {
		e.opts.Printer.Printf("cleaning archive %s...\n", e.opts.Archive)
		// os.RemoveAll(e.opts.Archive)
		e.opts.Printer.Printf("cleaning generation dir %s...\n", e.opts.BuildDir)
		if ok, err := vfs.DirExists(osfs.OsFs, e.opts.BuildDir); ok && err == nil {
			// err = os.RemoveAll(e.opts.BuildDir)
		}
	}
	return err
}
