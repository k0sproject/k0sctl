package cluster

import (
	"fmt"
	"net"
	"strings"

	"github.com/creasty/defaults"
	"github.com/jellydator/validation"
	"gopkg.in/yaml.v2"
)

// Spec defines cluster config spec section
type Spec struct {
	Hosts   Hosts   `yaml:"hosts,omitempty"`
	K0s     *K0s    `yaml:"k0s,omitempty"`
	Options Options `yaml:"options"`

	k0sLeader *Host
}

// UnmarshalYAML sets in some sane defaults when unmarshaling the data from yaml
func (s *Spec) UnmarshalYAML(unmarshal func(any) error) error {
	type spec Spec
	ys := (*spec)(s)
	ys.K0s = &K0s{}

	if err := unmarshal(ys); err != nil {
		return err
	}

	return defaults.Set(s)
}

// MarshalYAML overrides default YAML marshaling to get rid of "k0s: null" when nothing is set in spec.k0s
func (s *Spec) MarshalYAML() (any, error) {
	type spec Spec

	copy := spec(*s)

	if s.K0s == nil || isEmptyK0s(s.K0s) {
		copy.K0s = nil
	}

	return copy, nil
}

func isEmptyK0s(k *K0s) bool {
	if k == nil {
		return true
	}
	if k.Config != nil {
		return false
	}
	if k.Version != nil {
		return false
	}
	return len(k.Config) == 0
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

// ResolveUploadFilePaths resolves all host file sources relative to baseDir.
func (s *Spec) ResolveUploadFilePaths(baseDir string) error {
	for _, h := range s.Hosts {
		if err := h.ResolveUploadFiles(baseDir); err != nil {
			return err
		}
	}
	return nil
}

// Resolve prepares spec-level data after unmarshalling by cascading to hosts.
func (s *Spec) Resolve(baseDir string) error {
	return s.ResolveUploadFilePaths(baseDir)
}

type k0sCPLBConfig struct {
	Spec struct {
		Network struct {
			ControlPlaneLoadBalancing struct {
				Enabled    bool   `yaml:"enabled"`
				Type       string `yaml:"type"`
				Keepalived struct {
					VRRPInstances []struct {
						VirtualIPs []string `yaml:"virtualIPs"`
					} `yaml:"vrrpInstances"`
					VirtualServers []struct {
						IPAddress string `yaml:"ipAddress"`
					} `yaml:"virtualServers"`
				} `yaml:"keepalived"`
			} `yaml:"controlPlaneLoadBalancing"`
		} `yaml:"network"`
	} `yaml:"spec"`
}

// cplbConfig parses the keepalived control plane load balancing configuration
// from the k0s config. The second return value is false if the config can't be
// parsed or CPLB is not enabled with the Keepalived type.
func (s *Spec) cplbConfig() (*k0sCPLBConfig, bool) {
	if s.K0s == nil {
		return nil, false
	}

	cfg, err := yaml.Marshal(s.K0s.Config)
	if err != nil {
		return nil, false
	}

	k0scfg := &k0sCPLBConfig{}
	if err := yaml.Unmarshal(cfg, k0scfg); err != nil {
		return nil, false
	}

	cplb := k0scfg.Spec.Network.ControlPlaneLoadBalancing
	if !cplb.Enabled || cplb.Type != "Keepalived" {
		return nil, false
	}

	return k0scfg, true
}

// CPLBVIPs returns the set of control plane load balancing virtual IPs
// (keepalived virtualServers and vrrpInstances virtualIPs) declared in the k0s
// config. VRRP virtual IPs are included both in their raw configured form and,
// when configured in CIDR notation, as the bare IP address so that either form
// matches. Returns an empty set if CPLB is not enabled or not using Keepalived.
func (s *Spec) CPLBVIPs() map[string]struct{} {
	vips := make(map[string]struct{})

	k0scfg, ok := s.cplbConfig()
	if !ok {
		return vips
	}

	keepalived := k0scfg.Spec.Network.ControlPlaneLoadBalancing.Keepalived
	for _, vs := range keepalived.VirtualServers {
		if vs.IPAddress != "" {
			vips[vs.IPAddress] = struct{}{}
		}
	}
	for _, instance := range keepalived.VRRPInstances {
		for _, vipCIDR := range instance.VirtualIPs {
			if vipCIDR == "" {
				continue
			}
			// the value is not validated to be in CIDR notation, so keep the
			// raw form as well as the parsed bare IP to match either way
			vips[vipCIDR] = struct{}{}
			if vip, _, err := net.ParseCIDR(vipCIDR); err == nil {
				vips[vip.String()] = struct{}{}
			}
		}
	}

	return vips
}

func (s *Spec) clusterExternalAddress() string {
	if s.K0s != nil {
		if a := s.K0s.Config.DigString("spec", "api", "externalAddress"); a != "" {
			return a
		}

		if k0scfg, ok := s.cplbConfig(); ok {
			for _, vs := range k0scfg.Spec.Network.ControlPlaneLoadBalancing.Keepalived.VirtualServers {
				if addr := vs.IPAddress; addr != "" {
					return addr
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

// Resolve prepares spec-scoped resources after unmarshalling.
// Currently cascades resolution into hosts using the given origin.
func formatIPV6(address string) string {
	if strings.Contains(address, ":") {
		return fmt.Sprintf("[%s]", address)
	}
	return address
}
