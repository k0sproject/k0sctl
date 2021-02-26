package cluster

import "github.com/creasty/defaults"

// Spec defines cluster config spec section
type Spec struct {
	Hosts Hosts `yaml:"hosts" validate:"required,dive,min=1"`
	K0s   K0s   `yaml:"k0s"`

	k0sLeader *Host
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (s *Spec) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type spec Spec
	ys := (*spec)(s)

	if err := unmarshal(ys); err != nil {
		return err
	}

	return defaults.Set(s)
}

// K0sLeader returns a controller host that is selected to be a "leader",
// or an initial node, a node that creates join tokens for other controllers.
func (s *Spec) K0sLeader() *Host {
	if s.k0sLeader == nil {
		controllers := s.Hosts.Controllers()

		// Pick the first controller that reports to be running and persist the choice
		for _, h := range controllers {
			if h.Metadata.K0sBinaryVersion != "" && h.Metadata.K0sRunningVersion != "" {
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
