package phase

import (
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
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
		baseConfig.Spec.K0s.Config["spec"] = dig.Mapping{
			"network": dig.Mapping{
				"nodeLocalLoadBalancing": dig.Mapping{
					"enabled": true,
				},
			},
		}
		err := p.validateNodeLocalLoadBalancing()
		require.ErrorContains(t, err, "spec.k0s.config.network.nodeLocalLoadBalancing.enabled")
	})

	t.Run("passes when disabled on single", func(t *testing.T) {
		baseConfig.Spec.K0s.Config["spec"] = dig.Mapping{
			"network": dig.Mapping{
				"nodeLocalLoadBalancing": dig.Mapping{
					"enabled": false,
				},
			},
		}
		require.NoError(t, p.validateNodeLocalLoadBalancing())
	})

	t.Run("passes when not single", func(t *testing.T) {
		baseConfig.Spec.Hosts[0].Role = "controller"
		baseConfig.Spec.K0s.Config["spec"] = dig.Mapping{
			"network": dig.Mapping{
				"nodeLocalLoadBalancing": dig.Mapping{
					"enabled": true,
				},
			},
		}
		require.NoError(t, p.validateNodeLocalLoadBalancing())
	})
}

func TestWarnMultipleControllersWithoutLoadBalancer(t *testing.T) {
	controllers := func(n int) cluster.Hosts {
		var hosts cluster.Hosts
		for range n {
			hosts = append(hosts, &cluster.Host{Role: "controller"})
		}
		return append(hosts, &cluster.Host{Role: "worker"})
	}
	warnings := func(hosts cluster.Hosts, k0sConfig dig.Mapping) int {
		hook := logtest.NewLocal(log.StandardLogger())
		defer hook.Reset()
		p := &ValidateFacts{GenericPhase: GenericPhase{Config: &v1beta1.Cluster{Spec: &cluster.Spec{Hosts: hosts, K0s: &cluster.K0s{Config: k0sConfig}}}}}
		p.warnMultipleControllersWithoutLoadBalancer()
		n := 0
		for _, e := range hook.AllEntries() {
			if e.Level == log.WarnLevel {
				n++
			}
		}
		return n
	}

	t.Run("one controller", func(t *testing.T) {
		require.Zero(t, warnings(controllers(1), dig.Mapping{}))
	})

	t.Run("multiple controllers without a load balancer", func(t *testing.T) {
		require.NotZero(t, warnings(controllers(3), dig.Mapping{}))
	})

	t.Run("external address", func(t *testing.T) {
		cfg := dig.Mapping{"spec": dig.Mapping{"api": dig.Mapping{"externalAddress": "lb.example.com"}}}
		require.Zero(t, warnings(controllers(3), cfg))
	})

	t.Run("node-local load balancing", func(t *testing.T) {
		cfg := dig.Mapping{"spec": dig.Mapping{"network": dig.Mapping{"nodeLocalLoadBalancing": dig.Mapping{"enabled": true}}}}
		require.Zero(t, warnings(controllers(3), cfg))
	})
}
