package phase

import (
	"context"
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

	t.Run("uses the installed binary of a controller with useExistingK0s", func(t *testing.T) {
		// The configured version is one minor ahead, but the controller restarts on the installed v1.32 binary.
		h := running("controller", "v1.30.5+k0s.0")
		h.UseExistingK0s = true
		h.Metadata.NeedsUpgrade = true
		h.Metadata.K0sBinaryVersion = version.MustParse("v1.32.0+k0s.0")
		err := newPhase("v1.31.1+k0s.0", h).validateVersionSkew()
		require.ErrorContains(t, err, "from v1.30.5+k0s.0 to v1.32.0+k0s.0")

		// An installed binary one minor ahead is fine, even if the configured version is further ahead.
		h.Metadata.K0sBinaryVersion = version.MustParse("v1.31.1+k0s.0")
		require.NoError(t, newPhase("v1.32.0+k0s.0", h).validateVersionSkew())
	})

	t.Run("checks a worker with useExistingK0s against its installed binary", func(t *testing.T) {
		// The worker runs an old version, but its installed binary is within the supported skew, so there is
		// nothing to warn about; the check only warns and never fails for workers.
		h := running("worker", "v1.27.5+k0s.0")
		h.UseExistingK0s = true
		h.Metadata.NeedsUpgrade = true
		h.Metadata.K0sBinaryVersion = version.MustParse("v1.31.0+k0s.0")
		target := version.MustParse("v1.32.0+k0s.0")
		require.NoError(t, newPhase(target.String(), running("controller", "v1.31.5+k0s.0"), h).validateVersionSkew())
		require.True(t, postApplyVersion(h, target).Equal(version.MustParse("v1.31.0+k0s.0")))
	})

	t.Run("ignores the installed binary of a host with useExistingK0s that is not restarted", func(t *testing.T) {
		h := running("controller", "v1.30.5+k0s.0")
		h.UseExistingK0s = true
		h.Metadata.K0sBinaryVersion = version.MustParse("v1.32.0+k0s.0")
		require.NoError(t, newPhase("v1.30.5+k0s.0", h).validateVersionSkew())
	})

	t.Run("can be skipped", func(t *testing.T) {
		p := newPhase("v1.32.0+k0s.0", running("controller", "v1.30.5+k0s.0"))
		p.SkipVersionSkewCheck = true
		require.NoError(t, p.validateVersionSkew())
	})
}

func TestValidateFactsRun_VersionSkewWithUseExistingK0s(t *testing.T) {
	newPhase := func(defaulted bool) *ValidateFacts {
		h := &cluster.Host{Role: "controller", UseExistingK0s: true, Metadata: cluster.HostMetadata{
			K0sRunningVersion: version.MustParse("v1.30.5+k0s.0"),
			K0sBinaryVersion:  version.MustParse("v1.32.0+k0s.0"),
			NeedsUpgrade:      true,
		}}
		k0s := &cluster.K0s{Version: version.MustParse("v1.33.0+k0s.0")}
		k0s.Metadata.VersionDefaulted = defaulted
		return &ValidateFacts{GenericPhase: GenericPhase{Config: &v1beta1.Cluster{
			Spec: &cluster.Spec{Hosts: cluster.Hosts{h}, K0s: k0s},
		}}}
	}

	t.Run("blocks a restart on an installed binary that skips a minor version", func(t *testing.T) {
		require.ErrorContains(t, newPhase(false).Run(context.Background()), "from v1.30.5+k0s.0 to v1.32.0+k0s.0")
	})

	t.Run("allows a defaulted version that keeps the host on its running version", func(t *testing.T) {
		// The defaulted version is reset to the running version and the host is no longer upgraded,
		// so it is not restarted on the installed binary.
		require.NoError(t, newPhase(true).Run(context.Background()))
	})
}
