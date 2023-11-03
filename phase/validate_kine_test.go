package phase_test

import (
	"log"
	"os"
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	"github.com/stretchr/testify/require"
)

func TestNoKine(t *testing.T) {
	cfg := &v1beta1.Cluster{
		Metadata: &v1beta1.ClusterMetadata{
			Name: "k0s",
		},
		Spec: &cluster.Spec{
			K0s: &cluster.K0s{Config: dig.Mapping{}},
			Hosts: []*cluster.Host{
				{Role: "controller", Connection: rig.Connection{SSH: &rig.SSH{Address: "10.0.0.1", Port: 22}}},
			},
		},
	}

	p := &phase.ValidateKine{}
	require.NoError(t, p.Prepare(cfg))
	require.False(t, p.ShouldRun())
}

func TestKineSQLite(t *testing.T) {
	log.SetOutput(os.Stdout)

	cfg := &v1beta1.Cluster{
		Metadata: &v1beta1.ClusterMetadata{
			Name: "k0s",
		},
		Spec: &cluster.Spec{
			K0s: &cluster.K0s{
				Config: dig.Mapping{
					"storage": dig.Mapping{
						"type":    "kine",
						"storage": "",
					},
				},
			},
			Hosts: []*cluster.Host{
				{Role: "controller", Connection: rig.Connection{SSH: &rig.SSH{Address: "10.0.0.1", Port: 22}}},
			},
		},
	}

	p := &phase.ValidateKine{}
	require.NoError(t, p.Prepare(cfg))
	require.True(t, p.ShouldRun())
}
