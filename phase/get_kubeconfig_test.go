package phase

import (
	"context"
	"strings"
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/protocol/ssh"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
)

const ExpectedClusterAndContextName = "expected-context"
const ExpectedUserName = "expected-user"
const ExpectedClusterURL = "https://example.org"

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

// requireKubeConfigEqual compares two kubeconfig YAML strings semantically by
// parsing both, so the assertion is not coupled to clientcmd.Write formatting
// or field ordering, which can change across client-go versions.
func requireKubeConfigEqual(t *testing.T, expected, actual string) {
	t.Helper()
	expectedConfig, err := clientcmd.Load([]byte(expected))
	require.NoError(t, err)
	actualConfig, err := clientcmd.Load([]byte(actual))
	require.NoError(t, err)
	require.Equal(t, expectedConfig, actualConfig)
}

func TestGetKubeconfig(t *testing.T) {
	cfg := &v1beta1.Cluster{
		Metadata: &v1beta1.ClusterMetadata{
			Name: "k0s",
		},
		Spec: &cluster.Spec{
			K0s: &cluster.K0s{Config: dig.Mapping{}},
			Hosts: []*cluster.Host{
				{Role: "controller", CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: "10.0.0.1", Port: 22}}},
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
	actualOutput, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.NoError(t, err)
	requireKubeConfigEqual(t, expectedOutput, actualOutput)
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
	actualOutput, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.NoError(t, err)
	requireKubeConfigEqual(t, expectedOutput, actualOutput)
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
	actualOutput, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.NoError(t, err)
	requireKubeConfigEqual(t, expectedOutput, actualOutput)
}

func TestMissingCurrentContextFallsBackToSoleContext(t *testing.T) {
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
kind: Config
users:
- name: expected-user
  user: {}
`
	actualOutput, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.NoError(t, err)
	requireKubeConfigEqual(t, expectedOutput, actualOutput)
}

func TestMissingContext(t *testing.T) {
	input := `apiVersion: v1
current-context: test-context
kind: Config
`
	_, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.EqualError(t, err, "current context test-context not found in config")
}

func TestEmptyCurrentContextWithMultipleContexts(t *testing.T) {
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
kind: Config
`
	_, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.EqualError(t, err, "no current-context set and config does not contain exactly one context to fall back to")
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
	_, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.EqualError(t, err, "auth info test-user referenced by context test-context not found in config")
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
	_, err := kubeConfig(input, ExpectedClusterAndContextName, ExpectedClusterURL, ExpectedUserName)

	require.EqualError(t, err, "cluster test-cluster referenced by context test-context not found in config")
}
