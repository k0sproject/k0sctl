package cluster

import (
	"testing"

	"github.com/k0sproject/dig"
	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/protocol/ssh"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestKubeAPIURL(t *testing.T) {
	t.Run("with external address and port", func(t *testing.T) {
		spec := &Spec{
			K0s: &K0s{
				Config: dig.Mapping(map[string]any{
					"spec": dig.Mapping(map[string]any{
						"api": dig.Mapping(map[string]any{
							"port":            6444,
							"externalAddress": "test.example.com",
						}),
					}),
				}),
			}, Hosts: Hosts{
				&Host{
					Role: "controller",
					CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1"}},
				},
			},
		}
		require.Equal(t, "https://test.example.com:6444", spec.KubeAPIURL())
	})

	t.Run("without k0s config", func(t *testing.T) {
		spec := &Spec{
			Hosts: Hosts{
				&Host{
					Role:           "controller",
					PrivateAddress: "10.0.0.1",
					CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "192.168.0.1"}},
				},
			},
		}
		require.Equal(t, "https://192.168.0.1:6443", spec.KubeAPIURL())
	})

	t.Run("with CPLB", func(t *testing.T) {
		specYaml := []byte(`
hosts:
  - role: controller
    ssh:
      address: 192.168.0.1
    privateAddress: 10.0.0.1
k0s:
  config:
    spec:
      network:
        controlPlaneLoadBalancing:
          enabled: true
          type: Keepalived
          keepalived:
            vrrpInstances:
            - virtualIPs: ["192.168.0.10/24"]
              authPass: CPLB
            virtualServers:
            - ipAddress: 192.168.0.10`)

		spec := &Spec{}
		err := yaml.Unmarshal(specYaml, spec)
		require.NoError(t, err)

		require.Equal(t, "https://192.168.0.10:6443", spec.KubeAPIURL())
	})
}

// cplbSpec builds a Spec with a k0s config containing the given CPLB settings.
func cplbSpec(enabled bool, cplbType string, vrrpVIPs []string, virtualServerIPs []string) *Spec {
	vsEntries := make([]any, len(virtualServerIPs))
	for i, ip := range virtualServerIPs {
		vsEntries[i] = dig.Mapping{"ipAddress": ip}
	}
	vrrpEntries := make([]any, len(vrrpVIPs))
	for i, ip := range vrrpVIPs {
		vrrpEntries[i] = ip
	}
	return &Spec{
		K0s: &K0s{
			Config: dig.Mapping{
				"spec": dig.Mapping{
					"network": dig.Mapping{
						"controlPlaneLoadBalancing": dig.Mapping{
							"enabled": enabled,
							"type":    cplbType,
							"keepalived": dig.Mapping{
								"vrrpInstances":  []any{dig.Mapping{"virtualIPs": vrrpEntries}},
								"virtualServers": vsEntries,
							},
						},
					},
				},
			},
		},
	}
}

func TestCPLBVIPs(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		spec *Spec
		want bool // whether ip is reported as a CPLB VIP
	}{
		// nil / empty configs
		{
			name: "nil k0s",
			ip:   "10.0.0.1",
			spec: &Spec{},
			want: false,
		},
		{
			name: "nil config",
			ip:   "10.0.0.1",
			spec: &Spec{K0s: &K0s{}},
			want: false,
		},
		{
			name: "empty config",
			ip:   "10.0.0.1",
			spec: &Spec{K0s: &K0s{Config: dig.Mapping{}}},
			want: false,
		},
		{
			name: "invalid IP string",
			ip:   "not-an-ip",
			spec: cplbSpec(true, "Keepalived", nil, []string{"10.0.0.1"}),
			want: false,
		},
		{
			name: "empty IP string",
			ip:   "",
			spec: cplbSpec(true, "Keepalived", nil, []string{"10.0.0.1"}),
			want: false,
		},

		// CPLB disabled
		{
			name: "CPLB disabled, IP matches virtual server",
			ip:   "10.0.0.1",
			spec: cplbSpec(false, "Keepalived", nil, []string{"10.0.0.1"}),
			want: false,
		},
		{
			name: "CPLB disabled, IP matches VRRP plain VIP",
			ip:   "10.0.0.1",
			spec: cplbSpec(false, "Keepalived", []string{"10.0.0.1"}, nil),
			want: false,
		},
		{
			name: "CPLB disabled, IP matches VRRP CIDR",
			ip:   "10.0.0.1",
			spec: cplbSpec(false, "Keepalived", []string{"10.0.0.1/24"}, nil),
			want: false,
		},

		// CPLB enabled but wrong type
		{
			name: "CPLB enabled, type not Keepalived",
			ip:   "10.0.0.1",
			spec: cplbSpec(true, "Other", nil, []string{"10.0.0.1"}),
			want: false,
		},

		// Virtual server IPs
		{
			name: "first virtual server IP matches",
			ip:   "192.168.1.10",
			spec: cplbSpec(true, "Keepalived", nil, []string{"192.168.1.10", "192.168.1.11"}),
			want: true,
		},
		{
			name: "non-first virtual server IP matches",
			ip:   "192.168.1.11",
			spec: cplbSpec(true, "Keepalived", nil, []string{"192.168.1.10", "192.168.1.11"}),
			want: true,
		},
		{
			name: "virtual server IP no match",
			ip:   "192.168.1.99",
			spec: cplbSpec(true, "Keepalived", nil, []string{"192.168.1.10", "192.168.1.11"}),
			want: false,
		},

		// VRRP VIPs as plain IPs (direct string match)
		{
			name: "first VRRP plain IP matches",
			ip:   "10.0.0.1",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1", "10.0.0.2"}, nil),
			want: true,
		},
		{
			name: "non-first VRRP plain IP matches",
			ip:   "10.0.0.2",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1", "10.0.0.2"}, nil),
			want: true,
		},
		{
			name: "VRRP plain IP no match",
			ip:   "10.0.0.99",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1", "10.0.0.2"}, nil),
			want: false,
		},

		// VRRP VIPs as CIDRs (bare host address extracted)
		{
			name: "first VRRP CIDR host IP matches",
			ip:   "10.0.0.1",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1/24", "10.0.0.2/24"}, nil),
			want: true,
		},
		{
			name: "non-first VRRP CIDR host IP matches",
			ip:   "10.0.0.2",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1/24", "10.0.0.2/24"}, nil),
			want: true,
		},
		{
			name: "VRRP CIDR host IP no match",
			ip:   "10.0.0.99",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1/24", "10.0.0.2/24"}, nil),
			want: false,
		},
		{
			name: "VRRP CIDR network address does not match sibling host",
			ip:   "10.0.0.2",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1/24"}, nil),
			want: false,
		},

		// Invalid CIDR entries in VRRP list (should be skipped)
		{
			name: "invalid CIDR skipped, subsequent valid CIDR matches",
			ip:   "10.0.0.2",
			spec: cplbSpec(true, "Keepalived", []string{"bad-cidr", "10.0.0.2/24"}, nil),
			want: true,
		},
		{
			name: "invalid CIDR only, no match",
			ip:   "10.0.0.1",
			spec: cplbSpec(true, "Keepalived", []string{"bad-cidr"}, nil),
			want: false,
		},

		// No CPLB entries at all
		{
			name: "CPLB enabled Keepalived, no VRRPs and no virtual servers",
			ip:   "10.0.0.1",
			spec: cplbSpec(true, "Keepalived", nil, nil),
			want: false,
		},

		// Mixed: both VRRP and virtual servers present
		{
			name: "IP matches virtual server among mixed entries",
			ip:   "172.16.0.5",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1/24"}, []string{"172.16.0.5"}),
			want: true,
		},
		{
			name: "IP matches VRRP among mixed entries",
			ip:   "10.0.0.1",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1/24"}, []string{"172.16.0.5"}),
			want: true,
		},
		{
			name: "IP matches neither in mixed entries",
			ip:   "192.168.0.1",
			spec: cplbSpec(true, "Keepalived", []string{"10.0.0.1/24"}, []string{"172.16.0.5"}),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vips := tc.spec.CPLBVIPs()
			_, got := vips[tc.ip]
			require.Equal(t, tc.want, got)
		})
	}
}

// Without spec.k0s.config, the next apply recovers the cluster config from the
// leader, overrides included. The other controllers would then inherit them.
func TestSpecHostK0sConfigRequiresClusterK0sConfig(t *testing.T) {
	newSpec := func(k0s *K0s) *Spec {
		return &Spec{
			K0s: k0s,
			Hosts: Hosts{
				&Host{
					Role:            "controller",
					CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1"}},
					K0s:             &HostK0s{Config: dig.Mapping{"spec": dig.Mapping{"api": dig.Mapping{"address": "10.0.0.1"}}}},
				},
			},
		}
	}

	require.ErrorContains(t, newSpec(nil).ValidateHostK0sConfig(), "requires spec.k0s.config")
	require.ErrorContains(t, newSpec(&K0s{}).ValidateHostK0sConfig(), "requires spec.k0s.config")
	require.NoError(t, newSpec(&K0s{Config: dig.Mapping{"spec": dig.Mapping{}}}).ValidateHostK0sConfig())
}

// With dynamic config, the leader's first config seeds the cluster wide
// ClusterConfig, so the leader's overrides would reach the whole cluster.
func TestSpecHostK0sConfigRejectsDynamicConfig(t *testing.T) {
	spec := &Spec{
		K0s: &K0s{DynamicConfig: true, Config: dig.Mapping{"spec": dig.Mapping{}}},
		Hosts: Hosts{
			&Host{
				Role:            "controller",
				CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1"}},
				K0s:             &HostK0s{Config: dig.Mapping{"spec": dig.Mapping{"api": dig.Mapping{"address": "10.0.0.1"}}}},
			},
		},
	}
	require.ErrorContains(t, spec.ValidateHostK0sConfig(), "can't be used with dynamic config")
}

// k0sctl reads these keys only from the cluster config, so a host value
// would disagree with what k0sctl does.
func TestSpecHostK0sConfigRejectsClusterWideAPIKeys(t *testing.T) {
	for path, value := range map[[2]string]any{
		{"api", "externalAddress"}: "k8s.example.com",
		{"api", "port"}:            6444,
		{"storage", "type"}:        "kine",
	} {
		section, key := path[0], path[1]
		for _, v := range []any{value, nil} {
			spec := &Spec{
				K0s: &K0s{Config: dig.Mapping{"spec": dig.Mapping{}}},
				Hosts: Hosts{
					&Host{
						Role:            "controller",
						CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1"}},
						K0s:             &HostK0s{Config: dig.Mapping{"spec": dig.Mapping{section: dig.Mapping{key: v}}}},
					},
				},
			}
			require.ErrorContains(t, spec.ValidateHostK0sConfig(), "can't set spec."+section+"."+key)
		}
	}
}

func TestSpecHostK0sConfigRejectsDynamicConfigInstallFlag(t *testing.T) {
	h := &Host{
		Role:            "controller",
		CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1"}},
		K0s:             &HostK0s{Config: dig.Mapping{"spec": dig.Mapping{"api": dig.Mapping{"address": "10.0.0.1"}}}},
	}
	spec := &Spec{K0s: &K0s{Config: dig.Mapping{"spec": dig.Mapping{}}}, Hosts: Hosts{h}}
	require.NoError(t, spec.ValidateHostK0sConfig())

	h.InstallFlags.Add("--enable-dynamic-config")
	require.ErrorContains(t, spec.ValidateHostK0sConfig(), "can't be used with dynamic config")

	h.InstallFlags = Flags{"--enable-dynamic-config=false"}
	require.NoError(t, spec.ValidateHostK0sConfig())
}

// A null or scalar spec, spec.api or spec.storage would clear the cluster
// wide keys in the merge.
func TestSpecHostK0sConfigRejectsReplacedAPIParent(t *testing.T) {
	for _, config := range []dig.Mapping{
		{"spec": nil},
		{"spec": "x"},
		{"spec": dig.Mapping{"api": nil}},
		{"spec": dig.Mapping{"api": "x"}},
		{"spec": dig.Mapping{"storage": nil}},
		{"spec": dig.Mapping{"storage": "x"}},
	} {
		spec := &Spec{
			K0s: &K0s{Config: dig.Mapping{"spec": dig.Mapping{}}},
			Hosts: Hosts{
				&Host{
					Role:            "controller",
					CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1"}},
					K0s:             &HostK0s{Config: config},
				},
			},
		}
		require.ErrorContains(t, spec.ValidateHostK0sConfig(), "must be a mapping", config)
	}
}

func TestSpecHostK0sConfigAllowsEtcdPeerAddress(t *testing.T) {
	spec := &Spec{
		K0s: &K0s{Config: dig.Mapping{"spec": dig.Mapping{}}},
		Hosts: Hosts{
			&Host{
				Role:            "controller",
				CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1"}},
				K0s:             &HostK0s{Config: dig.Mapping{"spec": dig.Mapping{"storage": dig.Mapping{"etcd": dig.Mapping{"peerAddress": "172.16.0.1"}}}}},
			},
		},
	}
	require.NoError(t, spec.ValidateHostK0sConfig())
}
