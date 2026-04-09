package phase

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/configurer/windows"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/k0s/binprovider"
	"github.com/k0sproject/rig"
	rigos "github.com/k0sproject/rig/os"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalBinaryProviderCreatesParentDir(t *testing.T) {
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

	h := &genericHost{cluster.Host{
		Connection: rig.Connection{
			OSVersion: &rig.OSVersion{Name: "unknown", ID: "unknown"},
			Localhost: &rig.Localhost{Enabled: true},
		},
		K0sInstallPath: dest,
	}}
	h.SetSudofn(func(cmd string) string { return cmd })
	h.Connection.SetDefaults()
	require.NoError(t, h.Connect())
	require.NoError(t, h.ResolveConfigurer())

	h.K0sBinaryPath = src
	installPath := h.K0sInstallLocation()
	h.SetK0sBinaryProvider(binprovider.NewLocalFile(h, h.K0sBinaryPath, installPath, func() bool {
		return h.FileChanged(h.K0sBinaryPath, installPath)
	}))

	binaryProvider, err := h.K0sBinaryProvider(nil)
	require.NoError(t, err)
	tmpPath, err := binaryProvider.Stage(t.Context())
	require.NoError(t, err)

	h.Metadata.K0sBinaryTempFile = tmpPath

	entries, err := os.ReadDir(destDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, filepath.Join(destDir, entries[0].Name()), h.Metadata.K0sBinaryTempFile)

	if content, err := os.ReadFile(h.Metadata.K0sBinaryTempFile); assert.NoError(t, err) {
		assert.Equal(t, "test", string(content))
	}
}

type genericHost struct {
	cluster.Host
}

// Stub out OS detection parts
func (h *genericHost) ResolveConfigurer() error {
	switch runtime.GOOS {
	case "linux":
		h.OSVersion = &rig.OSVersion{Name: "linux", ID: "linux"}
		h.Configurer = &genericLinux{}
		return nil
	case "windows":
		h.OSVersion = &rig.OSVersion{Name: "windows", ID: "windows"}
		h.Configurer = &windows.Windows{}
		return nil
	}

	return errors.ErrUnsupported
}

type genericLinux struct {
	rigos.Linux
	linux.BaseLinux
}

func (*genericLinux) InstallPackage(rigos.Host, ...string) error {
	return errors.ErrUnsupported
}
