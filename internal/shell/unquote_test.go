package shell_test

import (
	"testing"

	"github.com/k0sproject/k0sctl/internal/shell"
	"github.com/stretchr/testify/require"
)

func TestUnquote(t *testing.T) {
	t.Run("no quotes", func(t *testing.T) {
		out, err := shell.Unquote("foo bar")
		require.NoError(t, err)
		require.Equal(t, "foo bar", out)
	})

	t.Run("simple quotes", func(t *testing.T) {
		out, err := shell.Unquote("\"foo\" 'bar'")
		require.NoError(t, err)
		require.Equal(t, "foo bar", out)
	})

	t.Run("mid-word quotes", func(t *testing.T) {
		out, err := shell.Unquote("f\"o\"o b'a'r")
		require.NoError(t, err)
		require.Equal(t, "foo bar", out)
	})

	t.Run("complex quotes", func(t *testing.T) {
		out, err := shell.Unquote(`'"'"'foo'"'"'`)
		require.NoError(t, err)
		require.Equal(t, `"'foo'"`, out)
	})

	t.Run("escaped quotes", func(t *testing.T) {
		out, err := shell.Unquote("\\'foo\\' 'bar'")
		require.NoError(t, err)
		require.Equal(t, "'foo' bar", out)
	})

	t.Run("windows path stays intact", func(t *testing.T) {
		out, err := shell.Unquote(`C:\var\lib\k0s`)
		require.NoError(t, err)
		require.Equal(t, `C:\var\lib\k0s`, out)
	})

	t.Run("escaped space", func(t *testing.T) {
		out, err := shell.Unquote(`foo\ bar`)
		require.NoError(t, err)
		require.Equal(t, "foo bar", out)
	})
}
