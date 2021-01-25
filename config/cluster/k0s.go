package cluster

import (
	"github.com/creasty/defaults"
	"github.com/k0sproject/k0sctl/integration/github"
	"github.com/k0sproject/k0sctl/version"
)

// K0s holds configuration for bootstraping a k0s cluster
type K0s struct {
	Version  string      `yaml:"version" validate:"required"`
	Config   Mapping     `yaml:"config"`
	Metadata K0sMetadata `yaml:"-"`
}

// K0sMetadata contains gathered information about k0s cluster
type K0sMetadata struct {
	ClusterID       string
	ControllerToken string
	WorkerToken     string
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (k *K0s) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type k0s K0s
	yk := (*k0s)(k)

	if err := unmarshal(yk); err != nil {
		return err
	}

	return defaults.Set(k)
}

// SetDefaults (implements defaults Setter interface) defaults the version to latest k0s version
func (k *K0s) SetDefaults() {
	if defaults.CanUpdate(k.Version) {
		preok := version.IsPre() || version.Version == "0.0.0"
		if latest, err := github.LatestK0sVersion(preok); err == nil {
			k.Version = latest
		}
	}
}
