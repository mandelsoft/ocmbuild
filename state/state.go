package state

import (
	"slices"

	"github.com/mandelsoft/goutils/general"
	"ocm.software/ocm/api/ocm/compdesc"
	metav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
	"ocm.software/ocm/api/ocm/compdesc/versions/ocm.software/v3alpha1"
	"ocm.software/ocm/cmds/ocm/commands/ocmcmds/common/addhdlrs/comp"

	"github.com/mandelsoft/ocm-build/buildfile"
)

type ResourceSpec struct {
	Source            string `json:"source"`
	comp.ResourceSpec `json:",inline"`
}

type Descriptor struct {
	State      map[string]interface{} `json:"state,omitempty"`
	BuildFile  *buildfile.Descriptor  `json:"buildfile,omitempty"`
	Components []*comp.ResourceSpec   `json:"components,omitempty"`
}

func New(buildfile *buildfile.Descriptor) *Descriptor {
	return &Descriptor{
		State:     map[string]interface{}{},
		BuildFile: buildfile,
	}
}

func (d *Descriptor) AddComponent(c *buildfile.Component) *comp.ResourceSpec {

	constructor := &comp.ResourceSpec{
		Meta: compdesc.Metadata{
			ConfiguredVersion: v3alpha1.SchemaVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:     c.Name,
			Version:  general.OptionalDefaulted(d.BuildFile.Version, c.Version),
			Labels:   MergeLabels(d.BuildFile.Labels, c.Labels),
			Provider: *MergeProvider(d.BuildFile.Provider, c.Provider),
		},
	}

	d.Components = append(d.Components, constructor)
	return constructor
}

func MergeProvider(a, b *metav1.Provider) *metav1.Provider {
	if b != nil {
		prov := a.Copy()
		if b.Name != "" {
			prov.Name = b.Name
		}
		prov.Labels = MergeLabels(prov.Labels, b.Labels)
	}
	if a == nil {
		return &metav1.Provider{}
	}
	return a
}

func MergeLabels(a, b metav1.Labels) metav1.Labels {
	result := slices.Clone(a)

	for _, l := range b {
		i := result.GetIndex(l.Name)
		if i >= 0 {
			result[i] = l
		} else {
			result = append(result, l)
		}
	}
	return result
}

type Element interface {
	GetRawIdentity() metav1.Identity
}

type Artifact interface {
	Element
	SetSourceFile(string)
}

func MergeArtifacts[E Artifact](a, b []E, src string) []E {
	return MergeElements(a, b, func(e E) { e.SetSourceFile(src) })
}

func MergeElements[E Element](a, b []E, mod ...func(e E)) []E {
	if len(b) != 0 {
		res := slices.Clone(a)
	outer:
		for _, r := range b {
			for _, m := range mod {
				m(r)
			}
			for i, o := range res {
				if o.GetRawIdentity().Equals(r.GetRawIdentity()) {
					res[i] = r
					continue outer
				}
			}
			res = append(res, r)
		}
		return res
	}
	return a
}
