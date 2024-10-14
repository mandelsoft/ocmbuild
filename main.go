package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/ocm/extensions/repositories/ctf"
	utils "ocm.software/ocm/api/ocm/ocmutils"
	"ocm.software/ocm/api/ocm/plugin/registration"
	"ocm.software/ocm/api/utils/template"

	"github.com/mandelsoft/ocm-build/build"
)

type Options struct {
	build.Options
	resolve bool
	clean   bool
}

func main() {

	var opts Options

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <archive> <buildfile>\n", os.Args[0]),
		Short: "compose an OCM transport archive from building a project",
		Long: "Described by a Buildfile, dome components are created ba executing arbitrary build steps\n" +
			"providing the resources included into the component version.",
		Example: "",
		Version: "0.1.0",
		RunE: func(cmd *cobra.Command, args []string) error {
			return Run(cmd, args, &opts)
		},
	}

	cmd.SetArgs(os.Args[1:])
	fs := cmd.Flags()

	fs.BoolVarP(&opts.ReResolve, "reresolve", "r", false, "reresolver plugin identities")
	fs.BoolVarP(&opts.Create, "create", "c", false, "create transprt archive")
	fs.BoolVarP(&opts.Force, "force", "f", false, "cleanup existing archive")
	fs.StringVarP(&opts.Archive, "target", "o", "", "target archive")
	fs.StringVarP(&opts.Version, "componentVersion", "V", "", "default version")
	fs.StringVarP(&opts.GenDir, "gen", "g", "gen", "generation directory")
	fs.StringVarP(&opts.PluginDir, "plugins", "p", "", "plugin di")
	fs.StringVarP(&opts.BuildFile, "buildfile", "b", "BuildFile.yaml", "build file")

	fs.BoolVarP(&opts.resolve, "resolve", "", false, "resolve used build plugins")
	fs.BoolVarP(&opts.clean, "clean", "", false, "clean build state")

	err := cmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: build failed: %s\n", os.Args[0], err.Error())
		os.Exit(1)
	}

}

func Run(cmd *cobra.Command, args []string, opts *Options) error {
	ctx := clictx.New()

	_, err := utils.Configure(ctx, "", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: configuration failed: %s\n", os.Args[0], err.Error())
		os.Exit(1)
	}
	registration.RegisterExtensions(ctx)
	opts.Format = ctf.FormatDirectory
	opts.Mode = 0o770
	opts.Templater = template.Options{
		Default: "spiff",
		UseEnv:  false,
	}

	if len(args) > 0 {
		opts.Components = args
	}

	if opts.clean {
		return build.Clean(ctx, opts.Options, !opts.resolve, true)
	}
	if opts.resolve {
		return build.Resolve(ctx, opts.Options)
	}
	return build.Execute(ctx, opts.Options)
}

func mainOld() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <archive> <constructor>\n", os.Args[0])
		os.Exit(1)
	}
	ctx := clictx.New()

	opts := build.Options{
		Create:  true,
		Archive: os.Args[1],
		Format:  ctf.FormatDirectory,
		Mode:    0o770,
		Version: "1.0.0",
	}
	err := build.Build(ctx, os.Args[2], opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		os.Exit(1)
	}
}
