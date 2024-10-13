package build

import (
	"github.com/spf13/pflag"
	clictx "ocm.software/ocm/api/cli"
	"ocm.software/ocm/api/ocm/extensions/repositories/ctf"
	"ocm.software/ocm/api/ocm/plugin/registration"
	"ocm.software/ocm/api/utils/accessio"
	"ocm.software/ocm/cmds/ocm/commands/common/options/formatoption"

	// bind OCM configuration.
	_ "ocm.software/ocm/api/ocm/plugin/ppi/config"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/options/templateroption"
	"ocm.software/ocm/cmds/test/build/build"

	"github.com/mandelsoft/logging"
	"github.com/spf13/cobra"

	"ocm.software/ocm/api/ocm"
)

const Name = "build"

var log = logging.DynamicLogger(logging.DefaultContext(), logging.NewRealm("cliplugin/rhabarber"))

func New() *cobra.Command {
	cmd := &command{}

	cmd.template.Options.Default = "spiff"
	cmd.format.Default = accessio.FormatDirectory

	c := &cobra.Command{
		Use:   Name + " <options> {<component>[:<version>]}",
		Short: "build project and generate transport archive",
		Long: `Based on a <code>BuildFile.yaml</code> described build plugins are executed
to generate artifacts from a source base and to generate components describing
thos artifacts. The component versions are stored in a transport archive.
`,
		RunE: cmd.Run,
	}

	cmd.AddFlags(c.Flags())
	return c
}

type command struct {
	opts    build.Options
	resolve bool
	clean   bool

	template templateroption.Option
	format   formatoption.Option
}

func (c *command) AddFlags(fs *pflag.FlagSet) {
	fs.BoolVarP(&c.opts.Create, "create", "c", false, "create transprt archive")
	fs.BoolVarP(&c.opts.Force, "force", "f", false, "cleanup existing archive")
	fs.StringVarP(&c.opts.Archive, "target", "o", "", "target archive")
	fs.StringVarP(&c.opts.Version, "componentVersion", "V", "", "default version")
	fs.StringVarP(&c.opts.GenDir, "gen", "g", "gen", "generation directory")
	fs.StringVarP(&c.opts.PluginDir, "plugins", "p", "", "plugin di")
	fs.StringVarP(&c.opts.BuildFile, "buildfile", "b", "BuildFile.yaml", "build file")

	fs.BoolVarP(&c.resolve, "resolve", "", false, "resolve used build plugins")
	fs.BoolVarP(&c.clean, "clean", "", false, "clean build state")

	c.template.AddFlags(fs)
	c.format.AddFlags(fs)
}

func (c *command) Run(cmd *cobra.Command, args []string) error {
	ctx := ocm.FromContext(cmd.Context())

	cctx := clictx.WithOCM(ctx).WithOutput(cmd.OutOrStdout()).WithErrorOutput(cmd.ErrOrStderr()).New()
	c.opts.Complete(cctx)

	log.Debug("running build command for %s", c.opts.BuildFile)

	registration.RegisterExtensions(ctx)

	err := c.template.Complete(cctx.FileSystem())
	if err != nil {
		return err
	}
	err = c.format.Configure(cctx)
	if err != nil {
		return err
	}

	c.opts.Format = ctf.GetFormat(c.format.Format)
	c.opts.Mode = c.format.Mode()
	c.opts.Templater = c.template.Options

	if len(args) > 0 {
		c.opts.Components = args
	}

	if c.clean {
		return build.Clean(cctx, c.opts, !c.resolve, true)
	}
	if c.resolve {
		return build.Resolve(cctx, c.opts)
	}
	return build.Execute(cctx, c.opts)
}
