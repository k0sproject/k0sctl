package phase

import (
	"context"
	"strings"
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

func fakeReader(h *cluster.Host) (string, error) {
	return strings.ReplaceAll(`apiVersion: v1
clusters:
- cluster:
    server: https://localhost:6443
  name: local
contexts:
- context:
    cluster: local
    user: user
  name: Default
current-context: Default
kind: Config
preferences: {}
users:
- name: user
  user:
`, "\t", "  "), nil
}

func TestGetKubeconfig(t *testing.T) {
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

	origReadKubeconfig := readKubeconfig
	defer func() { readKubeconfig = origReadKubeconfig }()
	readKubeconfig = fakeReader

	p := GetKubeconfig{GenericPhase: GenericPhase{Config: cfg}}
	require.NoError(t, p.Run(context.Background()))
	conf, err := clientcmd.Load([]byte(cfg.Metadata.Kubeconfig))
	require.NoError(t, err)
	require.Equal(t, "https://10.0.0.1:6443", conf.Clusters["k0s"].Server)

	cfg.Spec.Hosts[0].SSH.Address = "abcd:efgh:ijkl:mnop"
	p.APIAddress = ""
	require.NoError(t, p.Run(context.Background()))
	conf, err = clientcmd.Load([]byte(cfg.Metadata.Kubeconfig))
	require.NoError(t, err)
	require.Equal(t, "https://[abcd:efgh:ijkl:mnop]:6443", conf.Clusters["k0s"].Server)
}
