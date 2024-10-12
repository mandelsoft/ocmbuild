package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
	"github.com/mandelsoft/filepath/pkg/filepath"
	"github.com/mandelsoft/goutils/errors"
	"github.com/mandelsoft/goutils/general"
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
	ctx       ocm.Context
	path      string
	printer   common.Printer
	reresolve bool

	discovered []buildfile.Plugin
	plugins    []Entry
}

type Entry struct {
	Info
	path string
}

type Info struct {
	Id     HashId           `json:"id"`
	Spec   buildfile.Plugin `json:"spec"`
	Digest string           `json:"digest"`
}

type Plugin struct {
	path string
	desc string

	info Info
}

func (p *Plugin) Path() string {
	return p.path
}

func (p *Plugin) String() string {
	return fmt.Sprintf("%s[%s]", p.desc, vfs.Base(osfs.OsFs, p.path))
}

func New(ctx ocm.Context, path string, printer common.Printer, cached ...bool) (*PluginCache, error) {
	err := os.MkdirAll(path, 0o750)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot create build plugin dir")
	}

	var plugins []Entry

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".info") {
			var info Info
			pp := vfs.Join(osfs.OsFs, path, strings.TrimSuffix(e.Name(), ".info"))
			d, err := os.ReadFile(vfs.Join(osfs.OsFs, path, e.Name()))
			if err != nil {
				return nil, errors.Wrapf(err, "cannot read plugin info")
			}
			err = json.Unmarshal(d, &info)
			if err == nil {
				if ok, err := vfs.FileExists(osfs.OsFs, pp); ok && err == nil {
					plugins = append(plugins, Entry{info, pp})
				} else {
					os.Remove(e.Name())
					printer.Printf("WARNING: no ülgin found for %s in filesystem", e.Name(), err)
				}
			} else {
				printer.Printf("WARNING: cannot unmarshal plugin info for %s", e.Name(), err)
			}
		}
	}
	return &PluginCache{
		ctx:       ctx,
		path:      path,
		printer:   printer,
		plugins:   plugins,
		reresolve: !general.Optional(cached...),
	}, nil
}

type HashId struct {
	Component string
	Version   string
	Resource  string
}

func (i *HashId) IsComplete() bool {
	return i.Component != "" && i.Version != "" && i.Resource != ""
}

func (i *HashId) String() string {
	if i.Resource == "" {
		return fmt.Sprintf("%s:%s", i.Component, i.Version)
	}
	return fmt.Sprintf("%s:%s[%s]", i.Component, i.Version, i.Resource)
}

func Hash(id *HashId) string {
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

func (o *PluginCache) getPath(hid *HashId) string {
	return filepath.Join(o.path, Hash(hid))
}

func (o *PluginCache) add(info *Info, path string) *Plugin {
	o.discovered = append(o.discovered, info.Spec)
	o.plugins = append(o.plugins, Entry{
		Info: *info,
		path: path,
	})
	return &Plugin{
		path: path,
		desc: info.Id.String(),
		info: *info,
	}
}

func (o *PluginCache) Get(pspec *buildfile.Plugin, dir string) (*Plugin, error) {
	if pspec.Executable != "" {
		if pspec.PluginRef != "" {
			return nil, fmt.Errorf("for an execuable no reference required")
		}
		if pspec.Repository != nil {
			return nil, fmt.Errorf("for an execuable no repository required")
		}
		if pspec.Component != "" {
			return nil, fmt.Errorf("for an execuable no component required")
		}
		if pspec.Version != "" {
			return nil, fmt.Errorf("for an execuable no version required")
		}
		if pspec.Resource != "" {
			return nil, fmt.Errorf("for an execuable no resource required")
		}
		path := pspec.Executable
		if !vfs.IsAbs(osfs.OsFs, pspec.Executable) {
			path = vfs.Join(osfs.OsFs, dir, path)
		}
		return &Plugin{
			path: path,
			desc: "executable",
		}, nil
	}

	if pspec.Repository == nil && pspec.PluginRef == "" {
		return nil, fmt.Errorf("repository, reference or executable required")
	}
	if pspec.Repository != nil && pspec.PluginRef != "" {
		return nil, fmt.Errorf("either repository, reference or executable required")
	}

	var repospec ocm.RepositorySpec

	// in reresolve mode always reresolve the first occurrence of a ref, instead of resing
	// cached entry.
	discovered := !o.reresolve
	if !discovered {
		for _, s := range o.discovered {
			if reflect.DeepEqual(&s, pspec) {
				discovered = true
				break
			}
		}
	}

	// if assumed to be resolved, lookup cache entry
	if discovered {
		for _, pi := range o.plugins {
			if reflect.DeepEqual(&pi.Spec, pspec) {
				// o.printer.Printf("using cached plugin\n")
				return &Plugin{
					path: pi.path,
					desc: pi.Info.Id.String(),
				}, nil
			}
		}
	} else {
		for i, pi := range o.plugins {
			// forget cached resolution for yet undiscovered entry
			if reflect.DeepEqual(&pi.Spec, pspec) {
				// reevaluate ref
				o.plugins = append(o.plugins[:i], o.plugins[i+1:]...)
				break
			}
		}
	}

	info := Info{
		Spec: *pspec,
		Id: HashId{
			Component: pspec.Component,
			Version:   pspec.Version,
			Resource:  pspec.Resource,
		},
	}

	if pspec.PluginRef != "" {
		if pspec.Component != "" || pspec.Version != "" {
			return nil, fmt.Errorf("component or version not required for reference")
		}

		spec, err := ocm.ParseRef(pspec.PluginRef)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot parse reference spec")
		}
		if spec.Component != "" {
			if pspec.Component != "" || pspec.Version != "" {
				return nil, fmt.Errorf("component and version not required for given component reference")
			}
			info.Id.Component = spec.Component
			if spec.IsVersion() {
				info.Id.Version = *spec.Version
			}

		}
		repospec, err = o.ctx.MapUniformRepositorySpec(&spec.UniformRepositorySpec)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid repository spec")
		}
	}

	if pspec.Repository != nil {
		var raw interface{}
		err := json.Unmarshal(*pspec.Repository, &raw)
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
					if pspec.Component != "" {
						return nil, fmt.Errorf("component  not required for given component reference in repository field")
					}
					info.Id.Component = spec.Component
					if spec.IsVersion() {
						if pspec.Version != "" {
							return nil, fmt.Errorf("version not required for given version reference in repository field")
						}
						info.Id.Version = *spec.Version
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
			repospec, err = o.ctx.RepositorySpecForConfig(*pspec.Repository, nil)
			if err != nil {
				return nil, errors.Wrapf(err, "invalid repository spec")
			}
		}
	}

	if info.Id.IsComplete() {
		path := o.getPath(&info.Id)
		if ok, err := vfs.FileExists(osfs.OsFs, path); ok && err == nil {
			// o.printer.Printf("resusing completely specified ref\n")
			return o.add(&info, path), nil
		}
	}

	// New resolution for plugin spec
	sess := ocm.NewSession(nil)
	defer sess.Close()

	repo, err := sess.LookupRepository(o.ctx, repospec)
	if err != nil {
		return nil, errors.Wrapf(err, "cannot get repository")
	}

	return o.DownloadFromRepo(sess, repo, info.Id.Component, info.Id.Version, &info)
}

func (o *PluginCache) DownloadFromRepo(session ocm.Session, repo ocm.Repository, comp, vers string, info *Info) (*Plugin, error) {
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
			return o.download(session, cv, info.Id.Resource, info)
		}
		constraints, err := semver.NewConstraint(vers)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid version or constraints")
		}
		return o.downloadLatest(session, c, constraints, info.Id.Resource, info)

	}
	return o.downloadLatest(session, c, anyVersion, info.Id.Resource, info)
}

func (o *PluginCache) downloadLatest(session ocm.Session, comp ocm.ComponentAccess, constraints *semver.Constraints, res string, info *Info) (*Plugin, error) {
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
	return o.download(session, cv, res, info)
}

func (o *PluginCache) download(session ocm.Session, cv ocm.ComponentVersionAccess, name string, info *Info) (p *Plugin, err error) {
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

	hid := &HashId{
		Component: cv.GetName(),
		Version:   cv.GetVersion(),
		Resource:  found.Meta().Name,
	}

	target := o.getPath(hid)
	infofile := target + ".info"

	fs := osfs.New()
	if ok, _ := vfs.FileExists(fs, target); ok {
		var info Info
		d, err := os.ReadFile(infofile)
		if err == nil {
			err = json.Unmarshal(d, &info)
			if err != nil {
				return nil, err
			}
			if err == nil && info.Digest == found.Meta().Digest.Value {
				return o.add(&info, target), nil
			}
		}
	}

	printer := o.printer.AddGap("    ")
	d := ""
	if found.Meta().Digest != nil {
		d = "{" + found.Meta().Digest.Value + "}"
	}
	printer.Printf("found build plugin resource %s[%s:%s]%s\n", name, cv.GetName(), cv.GetVersion(), d)
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
	if found.Meta().Digest != nil {
		info.Digest = found.Meta().Digest.Value
		d, err := json.Marshal(info)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot marshal plugin info")
		}
		err = os.WriteFile(infofile, d, 0o644)
		if err != nil {
			return nil, errors.Wrapf(err, "cannot write plugin info file")
		}
	} else {
		os.Remove(infofile)
	}
	return o.add(info, target), nil
}
