package cluster

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	cfg "github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/configurer/linux"
	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/protocol/ssh"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

func TestHostK0sServiceName(t *testing.T) {
	h := Host{Role: "worker"}
	require.Equal(t, "k0sworker", h.K0sServiceName())
	h.Role = "controller"
	require.Equal(t, "k0scontroller", h.K0sServiceName())
	h.Role = "controller+worker"
	require.Equal(t, "k0scontroller", h.K0sServiceName())
}

func TestKubernetesNodeName(t *testing.T) {
	h := Host{Metadata: HostMetadata{Hostname: "IN291O-worker-0"}}
	require.Equal(t, "in291o-worker-0", h.KubernetesNodeName())
}

type mockconfigurer struct {
	cfg.Linux
	linux.Ubuntu
}

func (c *mockconfigurer) K0sCmdf(s string, args ...any) string {
	return fmt.Sprintf("k0s %s", fmt.Sprintf(s, args...))
}

func TestK0sJoinTokenPath(t *testing.T) {
	h := Host{}
	h.Configurer = &mockconfigurer{}
	h.Configurer.SetPath("K0sJoinTokenPath", "from-configurer")

	require.Equal(t, "from-configurer", h.K0sJoinTokenPath())

	h.InstallFlags.Add("--token-file from-install-flags")
	require.Equal(t, "from-install-flags", h.K0sJoinTokenPath())
}

func TestK0sConfigPath(t *testing.T) {
	h := Host{}
	h.Configurer = &mockconfigurer{}
	h.Configurer.SetPath("K0sConfigPath", "from-configurer")

	require.Equal(t, "from-configurer", h.K0sConfigPath())

	h.InstallFlags.Add("--config from-install-long-flag")
	require.Equal(t, "from-install-long-flag", h.K0sConfigPath())
	h.InstallFlags.Delete("--config")
	h.InstallFlags.Add("-c from-install-short-flag")
	require.Equal(t, "from-install-short-flag", h.K0sConfigPath())
}

func TestValidation(t *testing.T) {
	t.Run("installFlags", func(t *testing.T) {
		h := Host{
			Role:         "worker",
			InstallFlags: []string{"--foo"},
		}
		require.NoError(t, h.Validate())

		h.InstallFlags = []string{`--foo=""`, `--bar=''`}
		require.NoError(t, h.Validate())

		h.InstallFlags = []string{`--foo="`, "--bar"}
		require.ErrorContains(t, h.Validate(), "unbalanced quotes")

		h.InstallFlags = []string{"--bar='"}
		require.ErrorContains(t, h.Validate(), "unbalanced quotes")
	})
	t.Run("useExistingK0s", func(t *testing.T) {
		h := Host{Role: "worker", UseExistingK0s: true, UploadBinary: true}
		require.ErrorContains(t, h.Validate(), "uploadBinary cannot be true")
		h.UploadBinary = false
		h.K0sBinaryPath = "/tmp/k0s"
		require.ErrorContains(t, h.Validate(), "k0sBinaryPath cannot be set")
		h.K0sBinaryPath = ""
		h.K0sDownloadURLOverride = "https://example.test/k0s"
		require.ErrorContains(t, h.Validate(), "k0sDownloadURL cannot be set")
		h.K0sDownloadURLOverride = ""
		require.NoError(t, h.Validate())
	})
}

func TestBinaryPath(t *testing.T) {
	h := Host{}
	h.Configurer = &mockconfigurer{}
	h.Configurer.SetPath("K0sBinaryPath", "/foo/bar/k0s")
	require.Equal(t, "/foo/bar", h.k0sBinaryPathDir())
}

func TestResolveK0sBinaryPath(t *testing.T) {
	// Use t.TempDir() to get a real, OS-appropriate absolute path so tests pass on Windows too.
	baseDir := t.TempDir()

	t.Run("relative path is resolved against baseDir", func(t *testing.T) {
		h := Host{K0sBinaryPath: filepath.Join("bin", "k0s")}
		require.NoError(t, h.Resolve(baseDir))
		require.Equal(t, filepath.Join(baseDir, "bin", "k0s"), h.K0sBinaryPath)
	})
	t.Run("absolute path is kept as-is", func(t *testing.T) {
		absPath := filepath.Join(baseDir, "k0s")
		h := Host{K0sBinaryPath: absPath}
		require.NoError(t, h.Resolve(baseDir))
		require.Equal(t, absPath, h.K0sBinaryPath)
	})
	t.Run("empty baseDir leaves path unchanged", func(t *testing.T) {
		h := Host{K0sBinaryPath: filepath.Join("bin", "k0s")}
		require.NoError(t, h.Resolve(""))
		require.Equal(t, filepath.Join("bin", "k0s"), h.K0sBinaryPath)
	})
}

func TestExpandTokens(t *testing.T) {
	h := Host{
		Metadata: HostMetadata{
			Arch: "amd64",
		},
	}
	ver, err := version.NewVersion("v1.0.0+k0s.0")
	require.NoError(t, err)
	require.Equal(t, "test%20expand/k0s-v1.0.0%2Bk0s.0-amd64", h.ExpandTokens("test%20expand/k0s-%v-%p%x", ver))
}


// TestHost_ConnectLeavesRetryingToCaller verifies that Host.Connect makes a
// single attempt, so the retry loops around it see every failure. The rig
// client otherwise retries on its own for as long as the context allows.
func TestHost_ConnectLeavesRetryingToCaller(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	var accepted atomic.Int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			accepted.Add(1)
			_ = conn.Close()
		}
	}()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600))

	h := &Host{CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{
		Address:         "127.0.0.1",
		Port:            ln.Addr().(*net.TCPAddr).Port,
		User:            "test",
		KeyPath:         &keyPath,
		IgnoreSSHConfig: true,
	}}}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.Error(t, h.Connect(ctx))
	require.Equal(t, int32(1), accepted.Load())
}
