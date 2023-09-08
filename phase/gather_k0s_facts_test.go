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
}
