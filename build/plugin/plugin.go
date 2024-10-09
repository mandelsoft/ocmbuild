package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/mandelsoft/filepath/pkg/filepath"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/vfs/pkg/osfs"
	"github.com/mandelsoft/vfs/pkg/vfs"
	"ocm.software/ocm/api/ocm"
	"ocm.software/ocm/api/ocm/extensions/download"
	"ocm.software/ocm/api/ocm/extraid"
	"ocm.software/ocm/api/utils"
	common "ocm.software/ocm/api/utils/misc"
	"ocm.software/ocm/api/utils/semverutils"
	"ocm.software/ocm/cmds/test/build/buildfile"
)

const RESOURCE_TYPE = "ocm.software/buildplugin"

var anyVersion *semver.Constraints

func init() {
	anyVersion, _ = semver.NewConstraint("*")
}

type PluginCache struct {
	ctx     ocm.Context
	path    string
	printer common.Printer
}

type Plugin struct {
	path string
	desc string
}

func (p *Plugin) Path() string {
	return p.path
}

func (p *Plugin) String() string {
	return fmt.Sprintf("%s[%s]", p.desc, vfs.Base(osfs.OsFs, p.path))
}

func New(ctx ocm.Context, path string, printer common.Printer) (*PluginCache, error) {
	err := os.MkdirAll(path, 0o750)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create build plugin dir")
	}
	return &PluginCache{
		ctx:     ctx,
		path:    path,
		printer: printer,
	}, nil
}

type hashId struct {
	Component string
	Version   string
	Resource  string
}

func (i *hashId) String() string {
	if i.Resource == "" {
		return fmt.Sprintf("%s:%s", i.Component, i.Version)
	}
	return fmt.Sprintf("%s:%s[%s]", i.Component, i.Version, i.Resource)
}

func Hash(id *hashId) string {
	data, err := json.Marshal(id)
	if err != nil {
		panic(err)
	}
	data, err = jsoncanonicalizer.Transform(data)
	if err != nil {
		panic(err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (o *PluginCache) getPath(hid *hashId) string {
	return filepath.Join(o.path, Hash(hid))
}

func (o *PluginCache) Get(id *buildfile.Plugin, dir string) (*Plugin, error) {
	if id.Executable != "" {
		if id.PluginRef != "" {
			return nil, fmt.Errorf("for an execuable no reference required")
		}
		if id.Repository != nil {
			return nil, fmt.Errorf("for an execuable no repository required")
		}
		if id.Component != "" {
			return nil, fmt.Errorf("for an execuable no component required")
		}
		if id.Version != "" {
			return nil, fmt.Errorf("for an execuable no version required")
		}
		if id.Resource != "" {
			return nil, fmt.Errorf("for an execuable no resource required")
		}
		path := id.Executable
		if !vfs.IsAbs(osfs.OsFs, id.Executable) {
			path = vfs.Join(osfs.OsFs, dir, path)
		}
		return &Plugin{
			path: path,
			desc: "executable",
		}, nil
	}

	if id.Repository == nil && id.PluginRef == "" {
		return nil, fmt.Errorf("repository, reference or executable required")
	}
	if id.Repository != nil && id.PluginRef != "" {
		return nil, fmt.Errorf("either repository, reference or executable required")
	}

	var repospec ocm.RepositorySpec

	hid := &hashId{
		Component: id.Component,
		Version:   id.Version,
		Resource:  id.Resource,
	}

	if id.PluginRef != "" {
		if id.Component != "" || id.Version != "" {
			return nil, fmt.Errorf("component or version not required for reference")
		}

		spec, err := ocm.ParseRef(id.PluginRef)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse reference spec")
		}
		if spec.Component != "" {
			if id.Component != "" || id.Version != "" {
				return nil, fmt.Errorf("component and version not required for given component reference")
			}
			hid.Component = spec.Component
			if spec.IsVersion() {
				hid.Version = *spec.Version
			}

		}
		repospec, err = o.ctx.MapUniformRepositorySpec(&spec.UniformRepositorySpec)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid repository spec")
		}
	}

	if id.Repository != nil {
		var raw interface{}
		err := json.Unmarshal(*id.Repository, &raw)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse repository spec")
		}
		if s, ok := raw.(string); ok {
			var spec ocm.RefSpec
			if strings.Contains(s, "//") {
				spec, err = ocm.ParseRef(s)
				if err != nil {
					return nil, errors.Wrapf(err, "cannot parse repository spec")
				}
				if spec.Component != "" {
					if id.Component != "" {
						return nil, fmt.Errorf("component  not required for given component reference in repository field")
					}
					hid.Component = spec.Component
					if spec.IsVersion() {
						if id.Version != "" {
							return nil, fmt.Errorf("version not required for given version reference in repository field")
						}
						hid.Version = *spec.Version
					}
				}
			} else {
				spec.UniformRepositorySpec, err = ocm.ParseRepo(s)
				if err != nil {
					return nil, errors.Wrapf(err, "invalid repository spec")
				}
			}
			repospec, err = o.ctx.MapUniformRepositorySpec(&spec.UniformRepositorySpec)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid repository spec")
			}
		} else {
			repospec, err = o.ctx.RepositorySpecForConfig(*id.Repository, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid repository spec")
			}
		}
	}

	path := o.getPath(hid)
	if ok, err := vfs.FileExists(osfs.OsFs, path); ok && err == nil {
		return &Plugin{
			path: path,
			desc: hid.String(),
		}, nil
	}

	sess := ocm.NewSession(nil)

	repo, err := sess.LookupRepository(o.ctx, repospec)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get repository")
	}

	return o.DownloadFromRepo(sess, repo, hid.Component, hid.Version, hid)
}

func (o *PluginCache) DownloadFromRepo(session ocm.Session, repo ocm.Repository, comp, vers string, hid *hashId) (*Plugin, error) {
	var cv ocm.ComponentVersionAccess

	c, err := session.LookupComponent(repo, comp)
	if err != nil {
		return nil, err
	}

	if vers != "" {
		_, err := semver.NewVersion(vers)
		if err == nil {
			cv, err = session.GetComponentVersion(c, vers)
			if err != nil {
				return nil, err
			}
			return o.download(session, cv, hid.Resource)
		}
		constraints, err := semver.NewConstraint(vers)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid version or constraints")
		}
		return o.downloadLatest(session, c, constraints, hid.Resource)

	}
	return o.downloadLatest(session, c, anyVersion, hid.Resource)
}

func (o *PluginCache) downloadLatest(session ocm.Session, comp ocm.ComponentAccess, constraints *semver.Constraints, res string) (*Plugin, error) {
	var vers []string

	vers, err := comp.ListVersions()
	if err != nil {
		return nil, errors.Wrapf(err, "cannot list versions for component %s", comp.GetName())
	}
	if len(vers) == 0 {
		return nil, errors.Wrapf(err, "no versions found for component %s", comp.GetName())
	}

	versions, err := semverutils.MatchVersionStrings(vers, constraints)
	if err != nil {
		return nil, fmt.Errorf("failed to match version strings for component %s: %w", comp.GetName(), err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions for component %s match the constraints", comp.GetName())
	}
	if len(versions) > 1 {
		versions = versions[len(versions)-1:]
	}

	cv, err := session.GetComponentVersion(comp, versions[0].Original())
	if err != nil {
		return nil, err
	}
	return o.download(session, cv, res)
}

func (o *PluginCache) download(session ocm.Session, cv ocm.ComponentVersionAccess, name string) (p *Plugin, err error) {
	defer errors.PropagateErrorf(&err, nil, "%s", common.VersionedElementKey(cv))

	var found ocm.ResourceAccess
	var wrong ocm.ResourceAccess

	for _, r := range cv.GetResources() {
		if name != "" && r.Meta().Name != name {
			continue
		}
		if r.Meta().Type == RESOURCE_TYPE {
			if r.Meta().ExtraIdentity.Get(extraid.ExecutableOperatingSystem) == runtime.GOOS &&
				r.Meta().ExtraIdentity.Get(extraid.ExecutableArchitecture) == runtime.GOARCH {
				found = r
				break
			}
			wrong = r
		} else {
			if name != "" {
				wrong = r
			}
		}
	}
	if found == nil {
		if wrong != nil {
			if wrong.Meta().Type != RESOURCE_TYPE {
				return nil, fmt.Errorf("resource %q has wrong type: %s", wrong.Meta().Name, wrong.Meta().Type)
			}
			return nil, fmt.Errorf("os %s architecture %s not found for resource %q", runtime.GOOS, runtime.GOARCH, wrong.Meta().Name)
		}
		if name != "" {
			return nil, fmt.Errorf("resource %q not found", name)
		}
		return nil, fmt.Errorf("no ocm build plugin found")
	}

	hid := &hashId{
		Component: cv.GetName(),
		Version:   cv.GetVersion(),
		Resource:  found.Meta().Name,
	}
	target := o.getPath(hid)

	fs := osfs.New()
	if ok, _ := vfs.FileExists(fs, target); ok {
		return &Plugin{
			path: target,
			desc: hid.String(),
		}, nil
	}

	printer := o.printer.AddGap("    ")
	printer.Printf("found build plugin resource %s[%s:%s]\n", name, cv.GetName(), cv.GetVersion())
	file, err := os.CreateTemp(os.TempDir(), "plugin-*")
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create temp file")
	}
	file.Close()

	_, _, err = download.For(o.ctx).DownloadAsBlob(printer.AddGap("  "), found, file.Name(), fs)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot download resource %s", found.Meta().Name)
	}

	printer.Printf("installing build plugin %s[%s:%s] in %s...\n", name, hid.Component, hid.Version, o.path)
	dst, err := fs.OpenFile(target, vfs.O_CREATE|vfs.O_TRUNC|vfs.O_WRONLY, 0o755)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create plugin file %s", target)
	}
	src, err := fs.OpenFile(file.Name(), vfs.O_RDONLY, 0)
	if err != nil {
		dst.Close()
		return nil, errors.Wrapf(err, "cannot open plugin executable %s", file.Name())
	}
	_, err = io.Copy(dst, src)
	utils.IgnoreError(dst.Close())
	utils.IgnoreError(src.Close())
	utils.IgnoreError(os.Remove(file.Name()))
	if err != nil {
		return nil, errors.Wrapf(err, "cannot copy plugin file %s", target)
	}
	return &Plugin{
		path: target,
		desc: hid.String(),
	}, nil
}
