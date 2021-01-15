package config

import (
	"github.com/k0sproject/k0sctl/config/generic"
)

// ClusterConfig describes k0s.yaml configuration
type ClusterConfig struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   *ClusterMeta `yaml:"metadata"`
	Spec       *ClusterSpec `yaml:"spec"`
}

// ClusterMeta defines cluster metadata
type ClusterMeta struct {
	Name string `yaml:"name"`
}

// ClusterSpec defines cluster spec
type ClusterSpec struct {
	Hosts Hosts     `yaml:"hosts"`
	K0s   K0sConfig `yaml:"k0s,omitempty"`

	k0sLeader *Host
}

// K0sConfig holds configuration for bootstraping k0s cluster
type K0sConfig struct {
	Version  string              `yaml:"version"`
	Config   generic.GenericHash `yaml:"config"`
	Metadata K0sMetadata         `yaml:"-"`
}

// K0sMetadata information about k0s cluster install information
type K0sMetadata struct {
	Installed        bool
	InstalledVersion string
	ClusterID        string
	JoinToken        string
}
