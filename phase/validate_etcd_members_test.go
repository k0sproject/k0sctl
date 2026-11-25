package phase

import (
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/stretchr/testify/require"
)

func TestValidateControllerSwapUsesHostEtcdPeerAddress(t *testing.T) {
	h := testController("203.0.113.1", "10.0.0.1")
	h.K0s = &cluster.HostK0s{Config: dig.Mapping{
		"spec": dig.Mapping{"storage": dig.Mapping{"etcd": dig.Mapping{"peerAddress": "172.16.0.1"}}},
	}}

	config := &v1beta1.Cluster{
		Metadata: &v1beta1.ClusterMetadata{EtcdMembers: []string{"172.16.0.1"}},
		Spec:     &cluster.Spec{Hosts: cluster.Hosts{h}},
	}
	p := &ValidateEtcdMembers{GenericPhase: GenericPhase{Config: config}, hosts: cluster.Hosts{h}}

	err := p.validateControllerSwap()
	require.ErrorContains(t, err, "--peer-address 172.16.0.1")

	config.Metadata.EtcdMembers = []string{"10.0.0.1"}
	require.NoError(t, p.validateControllerSwap(), "the private address is not the peer address when the host overrides it")
}
