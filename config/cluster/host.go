package cluster

import (
	"fmt"

	"github.com/creasty/defaults"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// Host contains all the needed details to work with hosts
type Host struct {
	rig.Connection `yaml:",inline"`

	Role        string            `yaml:"role" validate:"oneof=server worker"`
	Environment map[string]string `yaml:"environment,flow,omitempty" default:"{}"`

	Metadata   HostMetadata `yaml:"-"`
	Configurer configurer
}

type configurer interface {
	CheckPrivilege() error
	StartService(string) error
	ServiceIsRunning(string) bool
	Arch() (string, error)
	K0sCmdf(string, ...interface{}) string
	K0sBinaryPath() string
	K0sConfigPath() string
	K0sJoinTokenPath() string
	RunK0sDownloader(string) error
}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sVersion string
	K0sRunning bool
	Arch       string
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (h *Host) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type host Host
	yh := (*host)(h)

	if err := unmarshal(yh); err != nil {
		return err
	}

	defaults.Set(h)

	return nil
}

func (h *Host) ResolveConfigurer() error {
	bf, err := registry.GetOSModuleBuilder(h.OSVersion)
	if err != nil {
		return err
	}
	if c, ok := bf(h).(configurer); ok {
		h.Configurer = c

		return nil
	}

	return fmt.Errorf("unsupported OS")
}
