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

func TestValidateRestore(t *testing.T) {
	newPhase := func(restoreFrom string, hosts ...*cluster.Host) *ValidateFacts {
		return &ValidateFacts{
			GenericPhase: GenericPhase{Config: &v1beta1.Cluster{Spec: &cluster.Spec{Hosts: hosts, K0s: &cluster.K0s{}}}},
			RestoreFrom:  restoreFrom,
		}
	}
	running := func(role string) *cluster.Host {
		h := &cluster.Host{Role: role}
		h.Metadata.K0sRunningVersion = version.MustParse("v1.33.1+k0s.0")
		return h
	}

	t.Run("fresh cluster", func(t *testing.T) {
		require.NoError(t, newPhase("backup.tar.gz", &cluster.Host{Role: "controller"}, &cluster.Host{Role: "worker"}).validateRestore())
	})

	t.Run("running controller", func(t *testing.T) {
		err := newPhase("backup.tar.gz", &cluster.Host{Role: "controller"}, running("controller+worker")).validateRestore()
		require.ErrorContains(t, err, "k0s is already running, a backup can only be restored to a new cluster")
	})

	t.Run("running worker only", func(t *testing.T) {
		require.NoError(t, newPhase("backup.tar.gz", &cluster.Host{Role: "controller"}, running("worker")).validateRestore())
	})

	t.Run("no restore", func(t *testing.T) {
		require.NoError(t, newPhase("", running("controller")).validateRestore())
	})
}
