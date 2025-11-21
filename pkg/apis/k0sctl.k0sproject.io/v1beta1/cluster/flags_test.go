package cluster

import (
	"testing"

	cfg "github.com/k0sproject/k0sctl/configurer"
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
	require.Equal(t, "--help --setting=false", flags.Join(&cfg.Linux{}))
}

func TestGetBoolean(t *testing.T) {
	t.Run("Valid flags", func(t *testing.T) {
		testsValid := []struct {
			flag   string
			expect bool
		}{
			{"--flag", true},
			{"--flag=true", true},
			{"--flag=false", false},
			{"--flag=1", true},
			{"--flag=TRUE", true},
		}
		for _, test := range testsValid {
			flags := Flags{test.flag}
			result, err := flags.GetBoolean(test.flag)
			require.NoError(t, err)
			require.Equal(t, test.expect, result)

			flags = Flags{"--unrelated-flag1", "--unrelated-flag2=foo", test.flag}
			result, err = flags.GetBoolean(test.flag)
			require.NoError(t, err)
			require.Equal(t, test.expect, result)
		}
	})

	t.Run("Invalid flags", func(t *testing.T) {
		testsInvalid := []string{
			"--flag=foo",
			"--flag=2",
			"--flag=TrUe",
			"--flag=-4",
			"--flag=FalSe",
		}
		for _, test := range testsInvalid {
			flags := Flags{test}
			_, err := flags.GetBoolean(test)
			require.Error(t, err)

			flags = Flags{"--unrelated-flag1", "--unrelated-flag2=foo", test}
			_, err = flags.GetBoolean(test)
			require.Error(t, err)
		}
	})

	t.Run("Unknown flags", func(t *testing.T) {
		flags := Flags{"--flag1=1", "--flag2"}
		result, err := flags.GetBoolean("--flag3")
		require.NoError(t, err)
		require.Equal(t, result, false)
	})
}

func TestEach(t *testing.T) {
	flags := Flags{"--flag1", "--flag2=foo", "--flag3=bar"}
	var countF, countV int
	flags.Each(func(flag string, value string) {
		countF++
		if value != "" {
			countV++
		}
	})
	require.Equal(t, 3, countF)
	require.Equal(t, 2, countV)
}

func TestMap(t *testing.T) {
	flags := Flags{"--flag1", "--flag2=foo", "--flag3=bar"}
	m := flags.Map()
	require.Len(t, m, 3)
	require.Equal(t, "", m["--flag1"])
	require.Equal(t, "foo", m["--flag2"])
	require.Equal(t, "bar", m["--flag3"])
}

func TestEquals(t *testing.T) {
	flags1 := Flags{"--flag1", "--flag2=foo", "--flag3=bar"}
	flags2 := Flags{"--flag1", "--flag2=foo", "--flag3=bar"}
	require.True(t, flags1.Equals(flags2))

	flags2 = Flags{"--flag1", "--flag2=foo"}
	require.False(t, flags1.Equals(flags2))

	flags2 = Flags{"-f", "--flag2=foo", "--flag3=baz"}
	require.False(t, flags1.Equals(flags2))
}

func TestNewFlags(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		flags, err := NewFlags("--hello=world --bar=baz")
		require.NoError(t, err)
		require.Equal(t, "world", flags.GetValue("--hello"))
		require.Equal(t, "baz", flags.GetValue("--bar"))
	})
	t.Run("empty", func(t *testing.T) {
		_, err := NewFlags("")
		require.NoError(t, err)
	})
}
