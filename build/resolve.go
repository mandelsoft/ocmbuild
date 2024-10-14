package build

import (
	"fmt"
	"strings"

	"github.com/mandelsoft/goutils/errors"
	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/utils/misc"

	"github.com/mandelsoft/ocm-build/buildfile"
)

func Resolve(ctx clictx.Context, opts Options) error {
	e, err := New(ctx, opts)
	if err != nil {
		return err
	}
	return e.Resolve()
}

func (e *Execution) Resolve() error {
	printer := e.opts.Printer
	printer.Printf("resolving build plugins...\n")
	printer = printer.AddGap("  ")

	if len(e.buildfile.Builds) > 0 {
		printer.Printf("resolving build steps....\n")
		err := e.ResolveBuilds(printer.AddGap("  "), e.buildfile.Builds, "")
		if err != nil {
			return err
		}
	}
	if len(e.buildfile.Components) > 0 {
		printer.Printf("resolving component build steps....\n")
		printer := printer.AddGap("  ")
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
			if c.Version == "" {
				c.Version = e.buildfile.Version
			}
			printer.Printf("resolving component %s...\n", misc.VersionedElementKey(&c))
			err := e.ResolveBuilds(printer.AddGap("  "), c.Builds, fmt.Sprintf("component %s, ", misc.VersionedElementKey(&c)))
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Execution) ResolveBuilds(printer misc.Printer, builds []buildfile.Build, ectx string) error {
	for i, b := range builds {
		p, err := e.plugins.Get(&b.Plugin, e.dir)
		if err != nil {
			return errors.Wrapf(err, "%sstep %d", ectx, i+1)
		}
		printer.Printf("step %d[%s]\n", i+1, p.String())

	}
	return nil
}
