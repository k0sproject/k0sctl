package phase

import (
	"context"
	"errors"
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	rigOS "github.com/k0sproject/rig/os"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

type privateAddressMock struct {
	mockconfigurer
	addr string
	err  error
}

func (m *privateAddressMock) PrivateAddress(_ rigOS.Host, _, _ string) (string, error) {
	return m.addr, m.err
}

// makeCPLBConfig builds a dig.Mapping representing a k0s config with CPLB settings.
func makeCPLBConfig(enabled bool, cplbType string, vrrpVIPs []string, virtualServerIPs []string) dig.Mapping {
	vsEntries := make([]any, len(virtualServerIPs))
	for i, ip := range virtualServerIPs {
		vsEntries[i] = dig.Mapping{"ipAddress": ip}
	}
	vrrpEntries := make([]any, len(vrrpVIPs))
	for i, ip := range vrrpVIPs {
		vrrpEntries[i] = ip
	}
	return dig.Mapping{
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
	}
}

func TestIsCPLBIP(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		config dig.Mapping
		want   bool
	}{
		// nil / invalid inputs
		{
			name:   "nil config",
			ip:     "10.0.0.1",
			config: nil,
			want:   false,
		},
		{
			name:   "empty config",
			ip:     "10.0.0.1",
			config: dig.Mapping{},
			want:   false,
		},
		{
			name:   "invalid IP string",
			ip:     "not-an-ip",
			config: makeCPLBConfig(true, "Keepalived", nil, []string{"10.0.0.1"}),
			want:   false,
		},
		{
			name:   "empty IP string",
			ip:     "",
			config: makeCPLBConfig(true, "Keepalived", nil, []string{"10.0.0.1"}),
			want:   false,
		},

		// CPLB absent from config
		{
			name: "no controlPlaneLoadBalancing section",
			ip:   "10.0.0.1",
			config: dig.Mapping{
				"spec": dig.Mapping{
					"network": dig.Mapping{},
				},
			},
			want: false,
		},

		// CPLB disabled
		{
			name:   "CPLB disabled, IP matches virtual server",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(false, "Keepalived", nil, []string{"10.0.0.1"}),
			want:   false,
		},
		{
			name:   "CPLB disabled, IP matches VRRP plain VIP",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(false, "Keepalived", []string{"10.0.0.1"}, nil),
			want:   false,
		},
		{
			name:   "CPLB disabled, IP matches VRRP CIDR",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(false, "Keepalived", []string{"10.0.0.1/24"}, nil),
			want:   false,
		},

		// CPLB enabled but wrong type
		{
			name:   "CPLB enabled, type not Keepalived",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(true, "Other", nil, []string{"10.0.0.1"}),
			want:   false,
		},

		// Virtual Server IPs
		{
			name:   "first virtual server IP matches",
			ip:     "192.168.1.10",
			config: makeCPLBConfig(true, "Keepalived", nil, []string{"192.168.1.10", "192.168.1.11"}),
			want:   true,
		},
		{
			name:   "non-first virtual server IP matches",
			ip:     "192.168.1.11",
			config: makeCPLBConfig(true, "Keepalived", nil, []string{"192.168.1.10", "192.168.1.11"}),
			want:   true,
		},
		{
			name:   "virtual server IP no match",
			ip:     "192.168.1.99",
			config: makeCPLBConfig(true, "Keepalived", nil, []string{"192.168.1.10", "192.168.1.11"}),
			want:   false,
		},

		// VRRP VIPs as plain IPs (direct string match)
		{
			name:   "first VRRP plain IP matches",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1", "10.0.0.2"}, nil),
			want:   true,
		},
		{
			name:   "non-first VRRP plain IP matches",
			ip:     "10.0.0.2",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1", "10.0.0.2"}, nil),
			want:   true,
		},
		{
			name:   "VRRP plain IP no match",
			ip:     "10.0.0.99",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1", "10.0.0.2"}, nil),
			want:   false,
		},

		// VRRP VIPs as CIDRs (host address extracted)
		{
			name:   "first VRRP CIDR host IP matches",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1/24", "10.0.0.2/24"}, nil),
			want:   true,
		},
		{
			name:   "non-first VRRP CIDR host IP matches",
			ip:     "10.0.0.2",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1/24", "10.0.0.2/24"}, nil),
			want:   true,
		},
		{
			name:   "VRRP CIDR host IP no match",
			ip:     "10.0.0.99",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1/24", "10.0.0.2/24"}, nil),
			want:   false,
		},
		// Network address of CIDR does not match a different host in that subnet
		{
			name:   "VRRP CIDR network address does not match sibling host",
			ip:     "10.0.0.2",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1/24"}, nil),
			want:   false,
		},

		// Invalid CIDR entries in VRRP list (should be skipped)
		{
			name:   "invalid CIDR skipped, subsequent valid CIDR matches",
			ip:     "10.0.0.2",
			config: makeCPLBConfig(true, "Keepalived", []string{"bad-cidr", "10.0.0.2/24"}, nil),
			want:   true,
		},
		{
			name:   "invalid CIDR only, no match",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(true, "Keepalived", []string{"bad-cidr"}, nil),
			want:   false,
		},

		// No CPLB entries at all
		{
			name:   "CPLB enabled Keepalived, no VRRPs and no virtual servers",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(true, "Keepalived", nil, nil),
			want:   false,
		},

		// Mixed: both VRRP and virtual servers present
		{
			name:   "IP matches virtual server among mixed entries",
			ip:     "172.16.0.5",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1/24"}, []string{"172.16.0.5"}),
			want:   true,
		},
		{
			name:   "IP matches VRRP among mixed entries",
			ip:     "10.0.0.1",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1/24"}, []string{"172.16.0.5"}),
			want:   true,
		},
		{
			name:   "IP matches neither in mixed entries",
			ip:     "192.168.0.1",
			config: makeCPLBConfig(true, "Keepalived", []string{"10.0.0.1/24"}, []string{"172.16.0.5"}),
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isCPLBIP(tc.ip, tc.config))
		})
	}
}

func TestInvestigateHostPrivateAddress(t *testing.T) {
	const iface = "eth0"

	makePhase := func(k0sConfig dig.Mapping) *GatherFacts {
		return &GatherFacts{
			GenericPhase: GenericPhase{
				Config: &v1beta1.Cluster{
					Spec: &cluster.Spec{
						K0s: &cluster.K0s{
							Version: version.MustParse("v1.33.0"),
							Config:  k0sConfig,
						},
					},
				},
			},
			SkipMachineIDs: true,
		}
	}

	makeHost := func(addr string, addrErr error) *cluster.Host {
		return &cluster.Host{
			HostnameOverride: "test-host",
			PrivateInterface: iface,
			Metadata:         cluster.HostMetadata{Arch: "amd64"},
			Configurer:       &privateAddressMock{addr: addr, err: addrErr},
		}
	}

	const cplbVIP = "10.0.0.1"
	const normalIP = "192.168.1.5"
	cplbCfg := makeCPLBConfig(true, "Keepalived", nil, []string{cplbVIP})

	tests := []struct {
		name        string
		phase       *GatherFacts
		host        *cluster.Host
		wantPrivate string
	}{
		{
			name:        "CPLB IP returned: private address not set",
			phase:       makePhase(cplbCfg),
			host:        makeHost(cplbVIP, nil),
			wantPrivate: "",
		},
		{
			name:        "non-CPLB IP returned: private address set",
			phase:       makePhase(cplbCfg),
			host:        makeHost(normalIP, nil),
			wantPrivate: normalIP,
		},
		{
			name:        "configurer error: private address not set",
			phase:       makePhase(cplbCfg),
			host:        makeHost("", errors.New("lookup failed")),
			wantPrivate: "",
		},
		{
			name:        "nil k0s config: non-CPLB check skipped, address set",
			phase:       makePhase(nil),
			host:        makeHost(normalIP, nil),
			wantPrivate: normalIP,
		},
		{
			name:        "CPLB disabled: configured VIP not treated as CPLB, address set",
			phase:       makePhase(makeCPLBConfig(false, "Keepalived", nil, []string{cplbVIP})),
			host:        makeHost(cplbVIP, nil),
			wantPrivate: cplbVIP,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.phase.investigateHost(context.Background(), tc.host))
			require.Equal(t, tc.wantPrivate, tc.host.PrivateAddress)
		})
	}
}
