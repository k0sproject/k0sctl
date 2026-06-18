package phase

import (
	"context"
	"errors"
	"testing"

	"github.com/k0sproject/dig"
	cfg "github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	rigOS "github.com/k0sproject/rig/os"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

type privateAddressMock struct {
	cfg.Linux
	linux.Ubuntu
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

func TestInvestigateHostPrivateAddress(t *testing.T) {
	const iface = "eth0"

	makePhase := func(k0sConfig dig.Mapping) *GatherFacts {
		p := &GatherFacts{
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
		// Run() precomputes this before investigateHost is called; mirror that
		// here since these tests exercise investigateHost directly.
		p.cplbVIPs = p.Config.Spec.CPLBVIPs()
		return p
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
