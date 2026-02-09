package phase

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploadK0sCreatesParentDir(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("No OS support module for darwin")
	}

	tmp := t.TempDir()

	src := filepath.Join(tmp, "k0s-src")
	require.NoError(t, os.WriteFile(src, []byte("test"), 0o600))

	destDir := filepath.Join(tmp, "nested", "bin")
	destName := "k0s"
	if runtime.GOOS == "windows" {
		destName = "k0s.exe"
	}
	dest := filepath.Join(destDir, destName)

	h := &cluster.Host{
		Connection:       rig.Connection{Localhost: &rig.Localhost{Enabled: true}},
		UploadBinaryPath: src,
		K0sInstallPath:   dest,
	}
	h.SetSudofn(func(cmd string) string { return cmd })
	h.Connection.SetDefaults()
	require.NoError(t, h.Connect())
	require.NoError(t, h.ResolveConfigurer())

	underTest := &UploadK0s{}
	require.NoError(t, underTest.uploadBinary(t.Context(), h))

	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Join(destDir, entries[0].Name()), h.Metadata.K0sBinaryTempFile)

	if content, err := os.ReadFile(h.Metadata.K0sBinaryTempFile); assert.NoError(t, err) {
		assert.Equal(t, "test", string(content))
	}
}
