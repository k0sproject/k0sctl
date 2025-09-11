package cluster

import (
	"fmt"
	"testing"

	cfg "github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
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

type mockconfigurer struct {
	cfg.Linux
	linux.Ubuntu
	quoteFn func(string) string
}

func (c *mockconfigurer) Quote(value string) string {
	if c.quoteFn != nil {
		return c.quoteFn(value)
	}
	return c.Linux.Quote(value)
}

func (c *mockconfigurer) Chown(_ os.Host, _, _ string, _ ...exec.Option) error {
	return nil
}

func (c *mockconfigurer) MkDir(_ os.Host, _ string, _ ...exec.Option) error {
	return nil
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

func TestK0sInstallCommand(t *testing.T) {
	h := Host{Role: "worker", DataDir: "/tmp/k0s", KubeletRootDir: "/tmp/kubelet", Connection: rig.Connection{Localhost: &rig.Localhost{Enabled: true}}}
	_ = h.Connect()
	h.Configurer = &mockconfigurer{}
	h.Configurer.SetPath("K0sConfigPath", "from-configurer")
	h.Configurer.SetPath("K0sJoinTokenPath", "from-configurer")

	cmd, err := h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --token-file=from-configurer`, cmd)

	h.Role = "controller"
	h.Metadata.IsK0sLeader = true
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --config=from-configurer`, cmd)

	h.Metadata.IsK0sLeader = false
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --token-file=from-configurer --config=from-configurer`, cmd)

	h.Role = "controller+worker"
	h.Metadata.IsK0sLeader = true
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --enable-worker=true --config=from-configurer`, cmd)

	h.Metadata.IsK0sLeader = false
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --enable-worker=true --token-file=from-configurer --config=from-configurer`, cmd)

	h.Role = "worker"
	h.PrivateAddress = "10.0.0.9"
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --token-file=from-configurer --kubelet-extra-args=--node-ip=10.0.0.9`, cmd)

	h.InstallFlags = []string{`--kubelet-extra-args="--foo bar"`}
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --kubelet-extra-args='--foo bar --node-ip=10.0.0.9' --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --token-file=from-configurer`, cmd)

	// Verify that K0sInstallCommand does not modify InstallFlags"
	require.Equal(t, `--kubelet-extra-args='--foo bar'`, h.InstallFlags.Join(h.Configurer))

	h.InstallFlags = []string{`--enable-cloud-provider=true`}
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --enable-cloud-provider=true --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --token-file=from-configurer`, cmd)
}

func TestK0sInstallCommandWindowsKubeletExtraArgs(t *testing.T) {
	h := Host{Role: "worker", DataDir: "/tmp/k0s", KubeletRootDir: "/tmp/kubelet", Connection: rig.Connection{Localhost: &rig.Localhost{Enabled: true}}}
	_ = h.Connect()
	winQuoter := &cfg.BaseWindows{}
	h.Configurer = &mockconfigurer{quoteFn: winQuoter.Quote}
	h.Configurer.SetPath("K0sConfigPath", "from-configurer")
	h.Configurer.SetPath("K0sJoinTokenPath", "from-configurer")
	h.PrivateAddress = "10.0.0.9"

	cmd, err := h.K0sInstallCommand()
	require.NoError(t, err)
	require.Contains(t, cmd, `--kubelet-extra-args=--node-ip=10.0.0.9`)
	require.NotContains(t, cmd, "`10.0.0.9`")
}

func TestK0sResetCommand(t *testing.T) {
	h := Host{Role: "worker", DataDir: "/tmp/k0s", KubeletRootDir: "/tmp/kubelet", Connection: rig.Connection{Localhost: &rig.Localhost{Enabled: true}}}
	_ = h.Connect()

	h.Configurer = &mockconfigurer{}
	require.Equal(t, `k0s reset --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet`, h.K0sResetCommand())
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

func TestFlagsChanged(t *testing.T) {
	cfg := &mockconfigurer{}
	cfg.SetPath("K0sConfigPath", "/tmp/foo.yaml")
	cfg.SetPath("K0sJoinTokenPath", "/tmp/token")
	t.Run("simple", func(t *testing.T) {
		h := Host{
			Configurer:     cfg,
			DataDir:        "/tmp/data",
			Role:           "controller",
			PrivateAddress: "10.0.0.1",
			InstallFlags:   []string{"--foo"},
			Metadata: HostMetadata{
				K0sStatusArgs: []string{"--foo", "--data-dir=/tmp/data", "--token-file=/tmp/token", "--config=/tmp/foo.yaml"},
			},
		}
		require.False(t, h.FlagsChanged())
		h.InstallFlags = []string{"--bar"}
		require.True(t, h.FlagsChanged())
	})
	t.Run("quoted values", func(t *testing.T) {
		h := Host{
			Configurer:     cfg,
			DataDir:        "/tmp/data",
			Role:           "controller+worker",
			PrivateAddress: "10.0.0.1",
			InstallFlags:   []string{"--foo='bar'", "--bar=foo"},
			Metadata: HostMetadata{
				K0sStatusArgs: []string{"--foo=bar", `--bar="foo"`, "--enable-worker=true", "--data-dir=/tmp/data", "--token-file=/tmp/token", "--config=/tmp/foo.yaml", "--kubelet-extra-args=--node-ip=10.0.0.1"},
			},
		}
		newFlags, err := h.K0sInstallFlags()
		require.NoError(t, err)
		require.False(t, h.FlagsChanged(), "flags %+v should not be considered different from %+v", newFlags, h.Metadata.K0sStatusArgs)
		h.InstallFlags = []string{"--foo=bar", `--bar="foo"`}
		require.False(t, h.FlagsChanged())
		h.InstallFlags = []string{"--foo=baz", `--bar="foo"`}
		require.True(t, h.FlagsChanged())
	})
	t.Run("kubelet-extra-args and single", func(t *testing.T) {
		h := Host{
			Configurer:     cfg,
			DataDir:        "/tmp/data",
			Role:           "single",
			PrivateAddress: "10.0.0.1",
			InstallFlags:   []string{"--foo='bar'", `--kubelet-extra-args="--bar=foo --foo='bar'"`},
			Metadata: HostMetadata{
				K0sStatusArgs: []string{"--foo=bar", `--kubelet-extra-args="--bar=foo --foo='bar'"`, "--data-dir=/tmp/data", "--single=true", "--token-file=/tmp/token", "--config=/tmp/foo.yaml"},
			},
		}
		flags, err := h.K0sInstallFlags()
		require.NoError(t, err)
		require.Equal(t, `--foo=bar --kubelet-extra-args='--bar=foo --foo='"'"'bar'"'"'' --data-dir=/tmp/data --single=true --token-file=/tmp/token --config=/tmp/foo.yaml`, flags.Join(h.Configurer))
		require.False(t, h.FlagsChanged())
		h.InstallFlags = []string{"--foo='baz'", `--kubelet-extra-args='--bar=baz --foo="bar"'`}
		flags, err = h.K0sInstallFlags()
		require.NoError(t, err)
		require.Equal(t, `--foo=baz --kubelet-extra-args='--bar=baz --foo="bar"' --data-dir=/tmp/data --single=true --token-file=/tmp/token --config=/tmp/foo.yaml`, flags.Join(h.Configurer))
		require.True(t, h.FlagsChanged())
	})
}
