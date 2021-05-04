package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildHosts(t *testing.T) {
	addresses := []string{
		"10.0.0.1",
		"",
		"10.0.0.2",
		"10.0.0.3",
	}
	hosts := buildHosts(addresses, 1, "test", "foo")
	require.Len(t, hosts, 3)
	require.Len(t, hosts.Controllers(), 1)
	require.Len(t, hosts.Workers(), 2)
	require.Equal(t, "test", hosts.First().SSH.User)
	require.Equal(t, "foo", hosts.First().SSH.KeyPath)

	hosts = buildHosts(addresses, 2, "", "")
	require.Len(t, hosts, 3)
	require.Len(t, hosts.Controllers(), 2)
	require.Len(t, hosts.Workers(), 1)
	require.Equal(t, "root", hosts.First().SSH.User)
	require.True(t, strings.HasSuffix(hosts.First().SSH.KeyPath, "/.ssh/id_rsa"))
}

func TestBuildHostsWithComments(t *testing.T) {
	addresses := []string{
		"# controllers",
		"10.0.0.1",
		"# workers",
		"10.0.0.2# second worker",
		"10.0.0.3 # last worker",
	}
	hosts := buildHosts(addresses, 1, "", "")
	require.Len(t, hosts, 3)
	require.Len(t, hosts.Controllers(), 1)
	require.Len(t, hosts.Workers(), 2)
	require.Equal(t, "10.0.0.1", hosts[0].Address())
	require.Equal(t, "10.0.0.2", hosts[1].Address())
	require.Equal(t, "10.0.0.3", hosts[2].Address())
}
