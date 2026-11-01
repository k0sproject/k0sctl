package phase

import (
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

func TestValidateNodeLocalLoadBalancing(t *testing.T) {
	baseConfig := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{{Role: "single"}},
			K0s:   &cluster.K0s{Config: dig.Mapping{}},
		},
	}

	p := &ValidateFacts{GenericPhase: GenericPhase{Config: baseConfig}}

	t.Run("fails when enabled on single", func(t *testing.T) {
		baseConfig.Spec.K0s.Config["network"] = dig.Mapping{
			"nodeLocalLoadBalancing": dig.Mapping{
				"enabled": true,
			},
		}
		err := p.validateNodeLocalLoadBalancing()
		require.ErrorContains(t, err, "spec.k0s.config.network.nodeLocalLoadBalancing.enabled")
	})

	t.Run("passes when disabled on single", func(t *testing.T) {
		baseConfig.Spec.K0s.Config["network"] = dig.Mapping{
			"nodeLocalLoadBalancing": dig.Mapping{
				"enabled": false,
			},
		}
		require.NoError(t, p.validateNodeLocalLoadBalancing())
	})

	t.Run("passes when not single", func(t *testing.T) {
		baseConfig.Spec.Hosts[0].Role = "controller"
		baseConfig.Spec.K0s.Config["network"] = dig.Mapping{
			"nodeLocalLoadBalancing": dig.Mapping{
				"enabled": true,
			},
		}
		require.NoError(t, p.validateNodeLocalLoadBalancing())
	})
}

func TestValidateVersionSkew(t *testing.T) {
	newPhase := func(target string, hosts ...*cluster.Host) *ValidateFacts {
		return &ValidateFacts{GenericPhase: GenericPhase{Config: &v1beta1.Cluster{
			Spec: &cluster.Spec{
				Hosts: hosts,
				K0s:   &cluster.K0s{Version: version.MustParse(target)},
			},
		}}}
	}
	running := func(role, v string) *cluster.Host {
		return &cluster.Host{Role: role, Metadata: cluster.HostMetadata{K0sRunningVersion: version.MustParse(v)}}
	}

	t.Run("allows a one minor version upgrade", func(t *testing.T) {
		p := newPhase("v1.31.1+k0s.0", running("controller", "v1.30.5+k0s.0"), running("worker", "v1.30.5+k0s.0"))
		require.NoError(t, p.validateVersionSkew())
	})

	t.Run("allows patch upgrades and fresh installs", func(t *testing.T) {
		p := newPhase("v1.31.2+k0s.0", running("controller", "v1.31.1+k0s.0"), &cluster.Host{Role: "controller"})
		require.NoError(t, p.validateVersionSkew())
	})

	t.Run("blocks a controller upgrade that skips a minor version", func(t *testing.T) {
		for _, role := range []string{"controller", "controller+worker", "single"} {
			p := newPhase("v1.32.0+k0s.0", running(role, "v1.30.5+k0s.0"))
			err := p.validateVersionSkew()
			require.ErrorContains(t, err, "one minor version at a time", role)
			require.ErrorContains(t, err, "upgrade to v1.31 first", role)
		}
	})

	t.Run("allows workers to skip minor versions", func(t *testing.T) {
		p := newPhase("v1.32.0+k0s.0", running("controller", "v1.31.5+k0s.0"), running("worker", "v1.29.5+k0s.0"))
		require.NoError(t, p.validateVersionSkew())
	})

	t.Run("ignores hosts that are being reset", func(t *testing.T) {
		h := running("controller", "v1.29.5+k0s.0")
		h.Reset = true
		p := newPhase("v1.32.0+k0s.0", running("controller", "v1.31.5+k0s.0"), h)
		require.NoError(t, p.validateVersionSkew())
	})

	t.Run("can be skipped", func(t *testing.T) {
		p := newPhase("v1.32.0+k0s.0", running("controller", "v1.30.5+k0s.0"))
		p.SkipVersionSkewCheck = true
		require.NoError(t, p.validateVersionSkew())
	})
}
