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

const ExpectedClusterAndContextName = "expected-context"
const ExpectedUserName = "expected-user"
const ExpectedClusterUrl = "https://example.org"

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

func TestConfigExtensionsRemain(t *testing.T) {
	input := `apiVersion: v1
clusters:
- cluster:
    server: https://localhost:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
extensions:
- extension:
    test: test
  name: test
kind: Config
users:
- name: test-user
  user: {}
`
	expectedOutput := `apiVersion: v1
clusters:
- cluster:
    server: https://example.org
  name: expected-context
contexts:
- context:
    cluster: expected-context
    user: expected-user
  name: expected-context
current-context: expected-context
extensions:
- extension:
    test: test
  name: test
kind: Config
users:
- name: expected-user
  user: {}
`
	actualOutput, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterUrl, ExpectedUserName)

	require.NoError(t, err)
	require.Equal(t, expectedOutput, actualOutput)
}

func TestContextExtensionsRemain(t *testing.T) {
	input := `apiVersion: v1
clusters:
- cluster:
    server: https://localhost:6443
  name: local-cluster
contexts:
- context:
    cluster: local-cluster
    extensions:
    - extension:
        test: test
      name: test
    user: user
  name: Default
current-context: Default
kind: Config
users:
- name: user
  user: {}
`
	expectedOutput := `apiVersion: v1
clusters:
- cluster:
    server: https://example.org
  name: expected-context
contexts:
- context:
    cluster: expected-context
    extensions:
    - extension:
        test: test
      name: test
    user: expected-user
  name: expected-context
current-context: expected-context
kind: Config
users:
- name: expected-user
  user: {}
`
	actualOutput, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterUrl, ExpectedUserName)

	require.NoError(t, err)
	require.Equal(t, expectedOutput, actualOutput)
}

func TestNonCurrentContextObjectsAreDropped(t *testing.T) {
	input := `apiVersion: v1
clusters:
- cluster:
    server: https://example.org
  name: test-cluster
- cluster:
    server: https://example.org
  name: test-cluster2
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
- context:
    cluster: test-cluster2
    user: test-user2
  name: test-context2
current-context: test-context
kind: Config
users:
- name: test-user
  user: {}
- name: test-user2
  user: {}
`
	expectedOutput := `apiVersion: v1
clusters:
- cluster:
    server: https://example.org
  name: expected-context
contexts:
- context:
    cluster: expected-context
    user: expected-user
  name: expected-context
current-context: expected-context
kind: Config
users:
- name: expected-user
  user: {}
`
	actualOutput, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterUrl, ExpectedUserName)

	require.NoError(t, err)
	require.Equal(t, expectedOutput, actualOutput)
}

func TestMissingContext(t *testing.T) {
	input := `apiVersion: v1
current-context: test-context
kind: Config
`
	_, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterUrl, ExpectedUserName)

	require.Errorf(t, err, "missing context should fail")
	require.Equal(t, err.Error(), "current context test-context not found in config")
}

func TestMissingAuthInfo(t *testing.T) {
	input := `apiVersion: v1
clusters:
- cluster:
    server: https://example.org
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
`
	_, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterUrl, ExpectedUserName)

	require.Errorf(t, err, "missing user should fail")
	require.Equal(t, err.Error(), "auth info test-user referenced by context test-context not found in config")
}

func TestMissingCluster(t *testing.T) {
	input := `apiVersion: v1
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
users:
- name: test-user
  user: {}
`
	_, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterUrl, ExpectedUserName)

	require.Errorf(t, err, "missing user should fail")
	require.Equal(t, err.Error(), "cluster test-cluster referenced by context test-context not found in config")
}
