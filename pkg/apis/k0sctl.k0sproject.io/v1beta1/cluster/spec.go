package cluster

import (
	"fmt"
	"strings"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
	"github.com/k0sproject/dig"
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

func (s *Spec) clusterExternalAddress() string {
	if s.K0s != nil {
		if a := s.K0s.Config.DigString("spec", "api", "externalAddress"); a != "" {
			return a
		}

		if cplb, ok := s.K0s.Config.Dig("spec", "network", "controlPlaneLoadBalancing").(dig.Mapping); ok {
			if enabled, ok := cplb.Dig("enabled").(bool); ok && enabled && cplb.DigString("type") == "Keepalived" {
				if virtualServers, ok := cplb.Dig("keepalived", "virtualServers").([]any); ok {
					for _, vs := range virtualServers {
						if vserver, ok := vs.(dig.Mapping); ok {
							if addr := vserver.DigString("ipAddress"); addr != "" {
								return addr
							}
						}
					}
				}
			}
		}
	}

	if leader := s.K0sLeader(); leader != nil {
		return leader.Address()
	}

	return ""
}

func (s *Spec) clusterInternalAddress() string {
	leader := s.K0sLeader()
	if leader.PrivateAddress != "" {
		return leader.PrivateAddress
	} else {
		return leader.Address()
	}
}

const defaultAPIPort = 6443

func (s *Spec) APIPort() int {
	if s.K0s != nil {
		if p, ok := s.K0s.Config.Dig("spec", "api", "port").(int); ok {
			return p
		}
	}
	return defaultAPIPort
}

// KubeAPIURL returns an external url to the cluster's kube API
func (s *Spec) KubeAPIURL() string {
	return fmt.Sprintf("https://%s:%d", formatIPV6(s.clusterExternalAddress()), s.APIPort())
}

// InternalKubeAPIURL returns a cluster internal url to the cluster's kube API
func (s *Spec) InternalKubeAPIURL() string {
	return fmt.Sprintf("https://%s:%d", formatIPV6(s.clusterInternalAddress()), s.APIPort())
}

// NodeInternalKubeAPIURL returns a cluster internal url to the node's kube API
func (s *Spec) NodeInternalKubeAPIURL(h *Host) string {
	addr := "127.0.0.1"

	// spec.api.onlyBindToAddress was introduced in k0s 1.30. Setting it to true will make the API server only
	// listen on the IP address configured by the `address` option.
	if onlyBindAddr, ok := s.K0s.Config.Dig("spec", "api", "onlyBindToAddress").(bool); ok && onlyBindAddr {
		if h.PrivateAddress != "" {
			addr = h.PrivateAddress
		} else {
			addr = h.Address()
		}
	}

	return fmt.Sprintf("https://%s:%d", formatIPV6(addr), s.APIPort())
}

func formatIPV6(address string) string {
	if strings.Contains(address, ":") {
		return fmt.Sprintf("[%s]", address)
	}
	return address
}
