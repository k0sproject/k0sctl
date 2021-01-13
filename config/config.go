package config

import (
	"github.com/k0sproject/k0sctl/config/generic"
	"github.com/k0sproject/rig"
	// "github.com/k0sproject/rig/client/local"
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

//Hosts are destnation hosts
type Hosts []*Host

// Host contains all the needed details to work with hosts
type Host struct {
	Role         string            `yaml:"role"`
	UploadBinary bool              `yaml:"uploadBinary,omitempty"`
	K0sBinary    string            `yaml:"k0sBinary,omitempty"`
	Environment  map[string]string `yaml:"environment,flow,omitempty" default:"{}"`

	// Address    string         `yaml:"address" validate:"required,hostname|ip"`
	// SSH        *ssh.Client    `yaml:"ssh,omitempty"`
	// Localhost  bool           `yaml:"localhost,omitempty"`
	Connection rig.Connection `yaml:"connection"`
	// WinRM      *winrm.Client  `yaml:"winRM,omitempty"`

	Metadata *HostMetadata `yaml:"-"`

	name string

	// Hooks        common.Hooks      `yaml:"hooks,omitempty" validate:"dive,keys,oneof=apply reset,endkeys,dive,keys,oneof=before after,endkeys,omitempty"`
	// InitSystem common.InitSystem `yaml:"-"`
	// Configurer HostConfigurer    `yaml:"-"`

}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sVersion string
	Arch       string
	// Os         *common.OsRelease
}
