package buildfile

import (
	"encoding/json"

	metav1 "ocm.software/ocm/api/ocm/compdesc/meta/v1"
)

type Descriptor struct {
	SchemaVersion string                 `json:"schemaVersion"`
	Metadata      map[string]interface{} `json:"metadata"`

	Version    string        `json:"version,omitempty"`
	Provider   *Provider     `json:"provider"`
	Labels     metav1.Labels `json:"labels,omitempty"`
	Builds     []Build       `json:"builds,omitempty"`
	Components []Component   `json:"components"`
}

type Provider = metav1.Provider

type Component struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`

	Provider *Provider     `json:"provider,omitempty"`
	Labels   metav1.Labels `json:"labels,omitempty"`

	Builds []Build `json:"builds"`
}

func (c *Component) GetName() string {
	return c.Name
}

func (c *Component) GetVersion() string {
	return c.Version
}

type Build struct {
	Plugin `json:",inline"`
	Config json.RawMessage `json:"config"`
}

type Plugin struct {
	PluginRef  string           `json:"pluginRef,omitempty"`
	Repository *json.RawMessage `json:"repository,omitempty"`
	Component  string           `json:"component,omitempty"`
	Version    string           `json:"version,omitempty"`
	Resource   string           `json:"resource,omitempty"`
	Executable *json.RawMessage `json:"executable,omitempty"`
}
