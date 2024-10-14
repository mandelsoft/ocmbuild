package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	runtime2 "runtime"
	"strings"
	"sync"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	metav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	v2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
	"ocm.software/ocm/api/ocm/extensions/artifacttypes"
	"ocm.software/ocm/api/utils"
	"ocm.software/ocm/api/utils/runtime"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/rscs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs/cpi"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs/types/docker"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs/types/dockermulti"

	"github.com/mandelsoft/ocm-build/ppi"
	"github.com/mandelsoft/ocm-build/state"
)

func main() {
	ppi.NewPlugin[Config](&Handler{}, usage).Run(os.Args)
}

type Config struct {
	Dockerfile  string   `json:"dockerfile"`
	ContentRoot string   `json:"contentRoot"`
	Options     []string `json:"options,omitempty"`

	Platforms []string `json:"platforms,omitempty""`
	Resource  Resource `json:"resource"`
}

type Resource struct {
	Name          string          `json:"name"`
	Type          string          `json:"type,omitempty"`
	ExtraIdentity metav1.Identity `json:"extraIdentity,omitempty"`
	ImageName     string          `json:"imageName,omitempty"`
	Labels        metav1.Labels   `json:"labels,omitempty"`
}

const usage = `
- name< (*string*) the (relative) the name for the OCM resource
- type (*string*) the resource type
- extraIdentity (*map[string]*) optional extra identity for the resource
- imageName (*string*) the reference hint for the generated image
- labels (*[]label*) arbitrary list of OCM labels
`

type Handler struct{}

var _ ppi.Handler[Config] = (*Handler)(nil)

func (h *Handler) Run(p *ppi.Plugin[Config], pstate *state.Descriptor, c *comp.ResourceSpec) error {
	config := p.Config()

	if config.Dockerfile == "" {
		return fmt.Errorf("dockerfile to build required")
	}
	if config.Resource.Name == "" {
		return fmt.Errorf("resource name required")
	}
	platforms := config.Platforms
	if len(config.Platforms) == 0 {
		platforms = []string{runtime2.GOOS + "/" + runtime2.GOARCH}
	}

	v := pstate.BuildFile.Version
	for _, pl := range config.Platforms {
		err := build(p, config, pl, v)
		if err != nil {
			return err
		}
	}
	err := apply(p, config, c, platforms, v)
	if err != nil {
		return err
	}

	return nil
}

func build(p *ppi.Plugin[Config], cfg *Config, platform, version string) error {

	dockerfile := p.Path(cfg.Dockerfile)

	target, err := ImageName(cfg.Resource.Name, platform, version)
	if err != nil {
		return err
	}

	root := cfg.ContentRoot
	if root == "" {
		root = vfs.Dir(osfs.OsFs, cfg.Dockerfile)
	}
	root = p.Path(root)
	args := append(append([]string{"buildx", "build", "--load", "-t", target, "--platform", platform, "--file", dockerfile}, cfg.Options...), root)
	if ok, err := vfs.Exists(osfs.OsFs, dockerfile); !ok || err != nil {
		return fmt.Errorf("dockerfile %q not found", dockerfile)
	}

	cmd := exec.Command("docker", args...)
	cmd.Stderr = os.Stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrapf(err, "cannot create pipe")
	}

	p.Printer().Printf("docker %s\n", strings.Join(args, " "))
	err = cmd.Start()
	if err != nil {
		return errors.Wrapf(err, "cannot run go build")
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go stream(wg, stdout, os.Stderr)

	err = cmd.Wait()
	if err != nil {
		return errors.Wrapf(err, "docker build failed")
	}
	wg.Wait()
	return nil
}

func stream(wg *sync.WaitGroup, in io.ReadCloser, out io.Writer) {
	var buf [8000]byte
	for {
		n, err := in.Read(buf[:])
		if err != nil || n < 0 {
			in.Close()
			wg.Done()
			return
		}
		out.Write(buf[:n])
	}
}

func apply(p *ppi.Plugin[Config], cfg *Config, c *comp.ResourceSpec, platforms []string, version string) error {
	var inp inputs.InputSpec

	var variants []string
	for _, p := range platforms {
		s, err := ImageName(cfg.Resource.Name, p, version)
		if err != nil {
			return err
		}
		variants = append(variants, s)
	}

	if len(platforms) == 1 {
		inp = &docker.Spec{
			PathSpec: cpi.PathSpec{
				InputSpecBase: inputs.InputSpecBase{
					ObjectVersionedType: runtime.ObjectVersionedType{
						Type: docker.TYPE,
					},
				},
				Path: variants[0],
			},
		}
	} else {
		inp = &dockermulti.Spec{
			InputSpecBase: inputs.InputSpecBase{
				ObjectVersionedType: runtime.ObjectVersionedType{
					Type: dockermulti.TYPE,
				},
			},
			Variants: variants,
		}
	}

	gen, err := inputs.ToGenericInputSpec(inp)
	if err != nil {
		return err
	}
	res := &rscs.ResourceSpec{
		ElementMeta: v2.ElementMeta{
			Name:          cfg.Resource.Name,
			ExtraIdentity: cfg.Resource.ExtraIdentity,
			Labels:        cfg.Resource.Labels,
		},
		Type:     utils.OptionalDefaulted(artifacttypes.OCI_IMAGE, cfg.Resource.Type),
		Relation: metav1.LocalRelation,
		ResourceInput: addhdlrs.ResourceInput{
			Input: gen,
		},
	}

	data, _ := json.Marshal(res)
	p.Printer().Printf("adding resource %s [%s]: %s\n", cfg.Resource.Name, cfg.Resource.ExtraIdentity.String(), string(data))
	c.Resources = state.MergeArtifacts(c.Resources, []*rscs.ResourceSpec{res}, "workdir")
	return nil
}

func ImageName(target string, platform string, version string) (string, error) {
	s := strings.Split(platform, "/")
	if len(s) != 2 {
		return "", fmt.Errorf("invalid platform %q", platform)
	}
	target += "-" + s[0] + "-" + s[1] + ":" + version
	return target, nil
}
