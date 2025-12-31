package phase

import (
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
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
