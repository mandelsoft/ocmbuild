package main

import (
	"os"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"ocm.software/ocm/api/utils/runtime"
	"ocm.software/ocm/api/utils/template"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"

	"github.com/mandelsoft/ocm-build/ppi"
	"github.com/mandelsoft/ocm-build/state"
)

func main() {
	ppi.NewPlugin[Config](&Handler{}, usage).Run(os.Args)
}

type Config struct {
	Constructor string                 `json:"constructor"`
	Values      map[string]interface{} `json:"values,omitempty"`
	Templater   string                 `json:"templater,omitempty"`
	UseEnv      bool                   `json:"useEnv,omitempty"`
}

const usage = `
- constructor< (*string*) the (relatice path to the constructor
  file
- values (*map*) arbitrary values passed to the templating of the
  constructor file.
- templater (*string*) the name of the templating engine to use
- useEnv (*bool*) pass environment variables to the templating engine.
`

type Handler struct{}

var _ ppi.Handler[Config] = (*Handler)(nil)

func (h *Handler) Run(p *ppi.Plugin[Config], pstate *state.Descriptor, c *comp.ResourceSpec) error {
	config := p.Config()

	if config.Constructor == "" {
		return errors.Newf("constructor required in plugin config")
	}

	templ := template.Options{
		Mode:   config.Templater,
		UseEnv: config.UseEnv,
		Vars:   config.Values,
	}

	err := templ.Complete(osfs.OsFs)
	if err != nil {
		return errors.Wrapf(err, "unknown templating engine")
	}

	constructor := p.Path(config.Constructor)
	cdata, err := os.ReadFile(constructor)
	if err != nil {
		return errors.Wrapf(err, "cannot read constructor %q[%s]", config.Constructor, constructor)
	}

	cproc, err := templ.Templater.Process(string(cdata), templ.Vars)
	if err != nil {
		return errors.Wrapf(err, "templating failed")
	}
	var res comp.ResourceSpec
	err = runtime.DefaultYAMLEncoding.Unmarshal([]byte(cproc), &res)
	if err != nil {
		return errors.Wrapf(err, "cannot run marshal constructor %q[%s]", config.Constructor, constructor)
	}

	if res.Name != "" {
		c.Name = res.Name
	}
	if res.Version != "" {
		c.Version = res.Version
	}
	c.Provider = *state.MergeProvider(&c.Provider, &res.Provider)
	c.Labels = state.MergeLabels(c.Labels, res.Labels)

	c.Resources = state.MergeArtifacts(c.Resources, res.Resources, constructor)
	c.Sources = state.MergeArtifacts(c.Sources, res.Sources, constructor)
	c.References = state.MergeElements(c.References, res.References)
	return nil
}
