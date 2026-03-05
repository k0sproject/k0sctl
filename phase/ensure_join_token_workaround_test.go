package phase

import (
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

// TestBuildDummyJoinTokenIsBase64 verifies that buildDummyJoinToken produces valid base64,
// which is what k0s checks when loading the join token file on v1.35.1+k0s.0.
func TestBuildDummyJoinTokenIsBase64(t *testing.T) {
	token, err := buildDummyJoinToken()
	require.NoError(t, err)
	require.True(t, isBase64(token), "buildDummyJoinToken must return valid base64")
}

func TestEnsureJoinTokenWorkaroundPrepare(t *testing.T) {
	workaroundVer := version.MustParse("v1.35.1+k0s.0")
	otherVer := version.MustParse("v1.34.0+k0s.0")
	newerVer := version.MustParse("v1.36.0+k0s.0")

	workerWithRunning := func(v *version.Version) *cluster.Host {
		return &cluster.Host{Role: "worker", Metadata: cluster.HostMetadata{K0sRunningVersion: v}}
	}
	workerNeedsUpgrade := func(targetVer *version.Version) (*cluster.Host, *v1beta1.Cluster) {
		h := &cluster.Host{Role: "worker", Metadata: cluster.HostMetadata{NeedsUpgrade: true}}
		cfg := &v1beta1.Cluster{
			Spec: &cluster.Spec{
				Hosts: cluster.Hosts{h},
				K0s:   &cluster.K0s{Version: targetVer},
			},
		}
		return h, cfg
	}

	t.Run("includes worker already running affected version", func(t *testing.T) {
		h := workerWithRunning(workaroundVer)
		cfg := &v1beta1.Cluster{
			Spec: &cluster.Spec{
				Hosts: cluster.Hosts{h},
				K0s:   &cluster.K0s{Version: workaroundVer},
			},
		}
		p := &EnsureJoinTokenWorkaround{}
		require.NoError(t, p.Prepare(cfg))
		require.Len(t, p.hosts, 1)
	})

	t.Run("excludes worker running a different version", func(t *testing.T) {
		h := workerWithRunning(otherVer)
		cfg := &v1beta1.Cluster{
			Spec: &cluster.Spec{
				Hosts: cluster.Hosts{h},
				K0s:   &cluster.K0s{Version: otherVer},
			},
		}
		p := &EnsureJoinTokenWorkaround{}
		require.NoError(t, p.Prepare(cfg))
		require.Empty(t, p.hosts)
	})

	t.Run("includes worker being upgraded to affected version", func(t *testing.T) {
		_, cfg := workerNeedsUpgrade(workaroundVer)
		p := &EnsureJoinTokenWorkaround{}
		require.NoError(t, p.Prepare(cfg))
		require.Len(t, p.hosts, 1)
	})

	t.Run("excludes worker being upgraded to a different version", func(t *testing.T) {
		_, cfg := workerNeedsUpgrade(newerVer)
		p := &EnsureJoinTokenWorkaround{}
		require.NoError(t, p.Prepare(cfg))
		require.Empty(t, p.hosts)
	})

	t.Run("excludes reset worker", func(t *testing.T) {
		h := workerWithRunning(workaroundVer)
		h.Reset = true
		cfg := &v1beta1.Cluster{
			Spec: &cluster.Spec{
				Hosts: cluster.Hosts{h},
				K0s:   &cluster.K0s{Version: workaroundVer},
			},
		}
		p := &EnsureJoinTokenWorkaround{}
		require.NoError(t, p.Prepare(cfg))
		require.Empty(t, p.hosts)
	})

	t.Run("excludes controllers", func(t *testing.T) {
		h := &cluster.Host{Role: "controller", Metadata: cluster.HostMetadata{K0sRunningVersion: workaroundVer}}
		cfg := &v1beta1.Cluster{
			Spec: &cluster.Spec{
				Hosts: cluster.Hosts{h},
				K0s:   &cluster.K0s{Version: workaroundVer},
			},
		}
		p := &EnsureJoinTokenWorkaround{}
		require.NoError(t, p.Prepare(cfg))
		require.Empty(t, p.hosts)
	})
}
