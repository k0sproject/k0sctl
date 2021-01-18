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

	Role         string            `yaml:"role" validate:"oneof=server worker"`
	UploadBinary bool              `yaml:"uploadBinary,omitempty"`
	K0sBinary    string            `yaml:"k0sBinary,omitempty" validate:"omitempty,file"`
	Environment  map[string]string `yaml:"environment,flow,omitempty" default:"{}"`

	Metadata   HostMetadata `yaml:"-"`
	Configurer configurer

	name string
}

type configurer interface {
	CheckPrivilege() error
	ServiceIsRunning(string) bool
}

// HostMetadata resolved metadata for host
type HostMetadata struct {
	K0sVersion string
	K0sRunning bool
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
