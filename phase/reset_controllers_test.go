package phase

import (
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/stretchr/testify/require"
)

func TestInstalledEtcdPeerAddress(t *testing.T) {
	h := testController("203.0.113.1", "10.0.0.1")
	h.K0s = &cluster.HostK0s{Config: dig.Mapping{
		"spec": dig.Mapping{"storage": dig.Mapping{"etcd": dig.Mapping{"peerAddress": "172.16.0.2"}}},
	}}

	require.Equal(t, "10.0.0.1", installedEtcdPeerAddress(h), "without an installed config, use the private address")

	h.Metadata.K0sExistingConfig = "spec:\n  storage:\n    etcd:\n      peerAddress: 172.16.0.1\n"
	require.Equal(t, "172.16.0.1", installedEtcdPeerAddress(h), "an unapplied host override must not replace the installed address")

	h.Metadata.K0sExistingConfig = "spec:\n  api:\n    port: 6443\n"
	require.Equal(t, "10.0.0.1", installedEtcdPeerAddress(h), "without an installed peerAddress, use the private address")
	h.PrivateAddress = ""
	require.Equal(t, "203.0.113.1", installedEtcdPeerAddress(h), "without a private address, use the connection address")
}
