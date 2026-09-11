package phase

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/download"
	"github.com/stretchr/testify/require"
)

func TestVerifyLocalSum(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "a.txt")
	require.NoError(t, os.WriteFile(name, []byte("hello"), 0o644))

	// sha256 of "hello"
	const sum = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	require.NoError(t, verifyLocalSum(name, sum))
	require.NoError(t, verifyLocalSum(name, "2CF24DBA5FB0A30E26E83B2AC5B9E29E1B161E5C1FA7425E73043362938B9824"))

	err := verifyLocalSum(name, "0000000000000000000000000000000000000000000000000000000000000000")
	require.ErrorIs(t, err, download.ErrChecksumMismatch)
	require.ErrorContains(t, err, sum)

	require.ErrorContains(t, verifyLocalSum(filepath.Join(dir, "missing.txt"), sum), "failed to open")
}
