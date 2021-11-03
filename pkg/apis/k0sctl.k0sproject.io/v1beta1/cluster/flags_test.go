package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlags(t *testing.T) {
	flags := Flags{"--admin-username=foofoo", "--san foo", "--ucp-insecure-tls"}
	require.Equal(t, "--ucp-insecure-tls", flags[2])
	require.Equal(t, 0, flags.Index("--admin-username"))
	require.Equal(t, 1, flags.Index("--san"))
	require.Equal(t, 2, flags.Index("--ucp-insecure-tls"))
	require.True(t, flags.Include("--san"))

	flags.Delete("--san")
	require.Equal(t, 1, flags.Index("--ucp-insecure-tls"))
	require.False(t, flags.Include("--san"))

	flags.AddOrReplace("--san 10.0.0.1")
	require.Equal(t, 2, flags.Index("--san"))
	require.Equal(t, "--san 10.0.0.1", flags.Get("--san"))
	require.Equal(t, "10.0.0.1", flags.GetValue("--san"))
	require.Equal(t, "foofoo", flags.GetValue("--admin-username"))

	require.Len(t, flags, 3)
	flags.AddOrReplace("--admin-password=barbar")
	require.Equal(t, 3, flags.Index("--admin-password"))
	require.Equal(t, "barbar", flags.GetValue("--admin-password"))

	require.Len(t, flags, 4)
	flags.AddUnlessExist("--admin-password=borbor")
	require.Len(t, flags, 4)
	require.Equal(t, "barbar", flags.GetValue("--admin-password"))

	flags.AddUnlessExist("--help")
	require.Len(t, flags, 5)
	require.True(t, flags.Include("--help"))
}

func TestFlagsWithQuotes(t *testing.T) {
	flags := Flags{"--admin-username \"foofoo\"", "--admin-password=\"foobar\""}
	require.Equal(t, "foofoo", flags.GetValue("--admin-username"))
	require.Equal(t, "foobar", flags.GetValue("--admin-password"))
}

func TestString(t *testing.T) {
	flags := Flags{"--help", "--setting=false"}
	require.Equal(t, "--help --setting=false", flags.Join())
}
