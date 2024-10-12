package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	metav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	v2 "ocm.software/ocm/api/ocm/compdesc/versions/v2"
	resourcetypes "ocm.software/ocm/api/ocm/extensions/artifacttypes"
	"ocm.software/ocm/api/ocm/extraid"
	"ocm.software/ocm/api/utils"
	"ocm.software/ocm/api/utils/mime"
	"ocm.software/ocm/api/utils/runtime"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/rscs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs/cpi"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/inputs/types/file"
	"ocm.software/ocm/cmds/test/build/ppi"
	"ocm.software/ocm/cmds/test/build/state"
)

func main() {
	ppi.NewPlugin[Config](&Handler{}).Run(os.Args)
}

type Config struct {
	Path    string   `json:"path"`
	Options []string `json:"options,omitempty"`

	Platforms []string `json:"platforms,omitempty""`
	Resource  Resource `json:"resource"`
}

type Resource struct {
	Name          string          `json:"name"`
	Type          string          `json:"type,omitempty"`
	ExtraIdentity metav1.Identity `json:"extraIdentity,omitempty"`
	Labels        metav1.Labels   `json:"labels,omitempty"`
}

type Handler struct{}

var _ ppi.Handler[Config] = (*Handler)(nil)

func (h *Handler) Run(p *ppi.Plugin[Config], pstate *state.Descriptor, c *comp.ResourceSpec) error {
	config := p.Config()

	if config.Path == "" {
		return fmt.Errorf("file path to build required")
	}
	if config.Resource.Name == "" {
		return fmt.Errorf("resource name required")
	}
	if len(config.Platforms) == 0 {
		t, id, err := build(p, config, "")
		if err != nil {
			return err
		}
		err = apply(p, config, c, t, id)
		if err != nil {
			return err
		}
	} else {
		for _, pl := range config.Platforms {
			t, id, err := build(p, config, pl)
			if err != nil {
				return err
			}
			err = apply(p, config, c, t, id)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func build(p *ppi.Plugin[Config], cfg *Config, platform string) (string, metav1.Identity, error) {
	target := p.GenDir(cfg.Resource.Name)

	var id metav1.Identity
	var info []string

	env := os.Environ()
	if platform != "" {
		s := strings.Split(platform, "/")
		if len(s) != 2 {
			return "", nil, fmt.Errorf("invalid platform %q", platform)
		}
		info = []string{"GOOS=" + s[0], "GOARCH=" + s[1]}
		env = append(env, info...)
		id = metav1.NewExtraIdentity(extraid.ExecutableOperatingSystem, s[0], extraid.ExecutableArchitecture, s[1])
		target += "-" + s[0] + "-" + s[1]
	}

	err := os.MkdirAll(vfs.Dir(osfs.OsFs, target), 0o755)
	if err != nil {
		return "", nil, err
	}
	args := append([]string{"build", "-o", target}, cfg.Options...)
	path := p.Path(cfg.Path)
	if ok, err := vfs.Exists(osfs.OsFs, path); !ok || err != nil {
		return "", nil, fmt.Errorf("path %q not found", path)
	}
	if !vfs.IsAbs(osfs.OsFs, path) {
		path = "." + string(os.PathSeparator) + path
	}
	args = append(args, path)
	cmd := exec.Command("go", args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr

	info = append(info, "go")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", nil, errors.Wrapf(err, "cannot create pipe")
	}

	p.Printer().Printf("%s\n", strings.Join(append(info, args...), " "))
	err = cmd.Start()
	if err != nil {
		return "", nil, errors.Wrapf(err, "cannot run go build")
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go stream(wg, stdout, os.Stderr)

	err = cmd.Wait()
	if err != nil {
		return "", nil, errors.Wrapf(err, "go build failed")
	}
	wg.Wait()
	return target, id, nil
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

func apply(p *ppi.Plugin[Config], cfg *Config, c *comp.ResourceSpec, target string, id metav1.Identity) error {
	extra := id.Copy()
	for k, v := range cfg.Resource.ExtraIdentity {
		extra[k] = v
	}

	inp, err := inputs.ToGenericInputSpec(&file.Spec{
		MediaFileSpec: cpi.MediaFileSpec{
			PathSpec: cpi.PathSpec{
				InputSpecBase: inputs.InputSpecBase{
					ObjectVersionedType: runtime.ObjectVersionedType{
						Type: file.TYPE,
					},
				},
				Path: target,
			},
			ProcessSpec: cpi.ProcessSpec{
				MediaType: mime.MIME_OCTET,
			},
		},
	})
	if err != nil {
		return err
	}

	res := &rscs.ResourceSpec{
		ElementMeta: v2.ElementMeta{
			Name:          cfg.Resource.Name,
			ExtraIdentity: extra,
			Labels:        cfg.Resource.Labels,
		},
		Type:     utils.OptionalDefaulted(resourcetypes.EXECUTABLE, cfg.Resource.Type),
		Relation: metav1.LocalRelation,
		ResourceInput: addhdlrs.ResourceInput{
			Input: inp,
		},
	}

	data, _ := json.Marshal(res)
	p.Printer().Printf("adding resource %s [%s]: %s\n", cfg.Resource.Name, id.String(), string(data))
	c.Resources = state.MergeArtifacts(c.Resources, []*rscs.ResourceSpec{res}, "workdir")
	return nil
}
