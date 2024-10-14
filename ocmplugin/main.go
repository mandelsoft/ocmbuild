package main

import (
	"os"

	// enable mandelsoft plugin logging configuration.
	_ "ocm.software/ocm/api/ocm/plugin/ppi/logging"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/names"

	"ocm.software/ocm/api/ocm/plugin/ppi"
	"ocm.software/ocm/api/ocm/plugin/ppi/clicmd"
	"ocm.software/ocm/api/ocm/plugin/ppi/cmds"
	"ocm.software/ocm/api/version"

	"github.com/mandelsoft/ocm-build/ocmplugin/cmds/build"
)

func main() {
	p := ppi.NewPlugin("ocmbuild", version.Get().String())

	p.SetShort("Component build command")
	p.SetLong("The plugin offers the component build plugin controlled by a <code>BuildFile.yaml</code>.")

	cmd, err := clicmd.NewCLICommand(build.New(), clicmd.WithCLIConfig(), clicmd.WithObjectType(names.Components[0]), clicmd.WithVerb(build.Name))
	if err != nil {
		os.Exit(1)
	}
	p.RegisterCommand(cmd)
	p.ForwardLogging()

	err = cmds.NewPluginCommand(p).Execute(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}
}
