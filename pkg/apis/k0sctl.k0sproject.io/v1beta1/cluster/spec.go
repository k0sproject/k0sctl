package cluster

import (
	"fmt"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
)

// Spec defines cluster config spec section
type Spec struct {
	Hosts Hosts `yaml:"hosts,omitempty"`
	K0s   *K0s  `yaml:"k0s,omitempty"`

	k0sLeader *Host
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (s *Spec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type spec Spec
	ys := (*spec)(s)
	ys.K0s = &K0s{}

	if err := unmarshal(ys); err != nil {
		return err
	}

	return defaults.Set(s)
}

// MarshalYAML implements yaml.Marshaler interface
func (s *Spec) MarshalYAML() (interface{}, error) {
	k0s, err := s.K0s.MarshalYAML()
	if err != nil {
		return nil, err
	}
	if k0s == nil {
		return Spec{Hosts: s.Hosts}, nil
	}

	return s, nil
}

// SetDefaults sets defaults
func (s *Spec) SetDefaults() {
	if s.K0s == nil {
		s.K0s = &K0s{}
		_ = defaults.Set(s.K0s)
	}
}

// K0sLeader returns a controller host that is selected to be a "leader",
// or an initial node, a node that creates join tokens for other controllers.
func (s *Spec) K0sLeader() *Host {
	if s.k0sLeader == nil {
		controllers := s.Hosts.Controllers()

		// Pick the first controller that reports to be running and persist the choice
		for _, h := range controllers {
			if !h.Reset && h.Metadata.K0sBinaryVersion != nil && h.Metadata.K0sRunningVersion != nil {
				s.k0sLeader = h
				break
			}
		}

		// Still nil?  Fall back to first "controller" host, do not persist selection.
		if s.k0sLeader == nil {
			return controllers.First()
		}
	}

	return s.k0sLeader
}

func (s *Spec) Validate() error {
	return validation.ValidateStruct(s,
		validation.Field(&s.Hosts, validation.Required),
		validation.Field(&s.Hosts),
		validation.Field(&s.K0s),
	)
}

// KubeAPIURL returns an url to the cluster's kube api
func (s *Spec) KubeAPIURL() string {
	var caddr string
	if a := s.K0s.Config.DigString("spec", "api", "externalAddress"); a != "" {
		caddr = a
	} else {
		leader := s.K0sLeader()
		if leader.PrivateAddress != "" {
			caddr = leader.PrivateAddress
		} else {
			caddr = leader.Address()
		}
	}

	cport := 6443
	if p, ok := s.K0s.Config.Dig("spec", "api", "port").(int); ok {
		cport = p
	}

	return fmt.Sprintf("https://%s:%d", caddr, cport)
}
