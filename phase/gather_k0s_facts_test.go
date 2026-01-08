package phase

import (
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

func TestNeedsUpgrade(t *testing.T) {
	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			K0s: &cluster.K0s{
				Version: version.MustParse("1.23.3+k0s.1"),
			},
		},
	}
	h := &cluster.Host{
		Metadata: cluster.HostMetadata{
			K0sRunningVersion: version.MustParse("1.23.3+k0s.1"),
		},
	}

	p := GatherK0sFacts{GenericPhase: GenericPhase{Config: cfg}}

	require.False(t, p.needsUpgrade(h))
	h.Metadata.K0sRunningVersion = version.MustParse("1.23.3+k0s.2")
	require.False(t, p.needsUpgrade(h))
	h.Metadata.K0sRunningVersion = version.MustParse("1.23.3+k0s.0")
	require.True(t, p.needsUpgrade(h))
	h.UseExistingK0s = true
	require.False(t, p.needsUpgrade(h))
}

func TestReportUseExistingHostsDryMessages(t *testing.T) {
	clusterVersion := version.MustParse("1.24.0+k0s.0")
	h := &cluster.Host{
		UseExistingK0s: true,
		Metadata: cluster.HostMetadata{
			K0sBinaryVersion: version.MustParse("1.23.0+k0s.0"),
		},
	}
	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{h},
			K0s:   &cluster.K0s{Version: clusterVersion},
		},
	}
	p := GatherK0sFacts{GenericPhase: GenericPhase{Config: cfg}}
	mgr := &Manager{DryRun: true, Config: cfg}
	p.SetManager(mgr)
	require.NoError(t, p.reportUseExistingHosts())
	require.Len(t, mgr.dryMessages, 1)
	for _, msgs := range mgr.dryMessages {
		require.Len(t, msgs, 2)
		require.Contains(t, msgs[0], "reuse existing k0s v1.23.0+k0s.0")
		require.Contains(t, msgs[1], "WARNING")
	}
}

func TestReportUseExistingHostsFailsWithoutBinary(t *testing.T) {
	h := &cluster.Host{UseExistingK0s: true}
	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{Hosts: cluster.Hosts{h}},
	}
	p := GatherK0sFacts{GenericPhase: GenericPhase{Config: cfg}}
	mgr := &Manager{Config: cfg}
	p.SetManager(mgr)
	require.ErrorContains(t, p.reportUseExistingHosts(), "useExistingK0s=true but no k0s binary found on host")
}

func TestHandleRoleMismatch(t *testing.T) {
	originalForce := Force
	t.Cleanup(func() { Force = originalForce })

	p := GatherK0sFacts{}

	t.Run("host not marked for reset", func(t *testing.T) {
		Force = true
		h := &cluster.Host{Role: "controller"}
		err := p.handleRoleMismatch(h, "worker")
		require.ErrorContains(t, err, "role change is not supported")
		require.Equal(t, "controller", h.Role)
	})

	t.Run("reset host requires force", func(t *testing.T) {
		Force = false
		h := &cluster.Host{Role: "controller", Reset: true}
		err := p.handleRoleMismatch(h, "single")
		require.ErrorContains(t, err, "use --force")
		require.Equal(t, "controller", h.Role)
	})

	t.Run("force allows role update", func(t *testing.T) {
		Force = true
		h := &cluster.Host{Role: "controller", Reset: true}
		require.NoError(t, p.handleRoleMismatch(h, "worker"))
		require.Equal(t, "worker", h.Role)
	})
}
