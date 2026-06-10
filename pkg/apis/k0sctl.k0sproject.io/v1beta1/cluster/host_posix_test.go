//go:build !windows

package cluster

import (
	"context"
	"testing"

	rig "github.com/k0sproject/rig/v2"
	"github.com/stretchr/testify/require"
)

func TestK0sInstallCommand(t *testing.T) {
	h := Host{Role: "worker", DataDir: "/tmp/k0s", KubeletRootDir: "/tmp/kubelet", CompositeConfig: rig.CompositeConfig{Localhost: rig.LocalhostConfig(true)}}
	_ = h.Connect(context.Background())
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
	require.Equal(t, `--kubelet-extra-args='--foo bar'`, h.InstallFlags.Join(h.FS()))

	h.InstallFlags = []string{`--enable-cloud-provider=true`}
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --enable-cloud-provider=true --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet --token-file=from-configurer`, cmd)
}

func TestK0sResetCommand(t *testing.T) {
	h := Host{Role: "worker", DataDir: "/tmp/k0s", KubeletRootDir: "/tmp/kubelet", CompositeConfig: rig.CompositeConfig{Localhost: rig.LocalhostConfig(true)}}
	_ = h.Connect(context.Background())

	h.Configurer = &mockconfigurer{}
	require.Equal(t, `k0s reset --data-dir=/tmp/k0s --kubelet-root-dir=/tmp/kubelet`, h.K0sResetCommand())
}

func TestFlagsChanged(t *testing.T) {
	mc := &mockconfigurer{}
	mc.SetPath("K0sConfigPath", "/tmp/foo.yaml")
	mc.SetPath("K0sJoinTokenPath", "/tmp/token")
	t.Run("simple", func(t *testing.T) {
		h := Host{
			CompositeConfig: rig.CompositeConfig{Localhost: rig.LocalhostConfig(true)},
			Configurer:      mc,
			DataDir:         "/tmp/data",
			Role:            "controller",
			PrivateAddress:  "10.0.0.1",
			InstallFlags:    []string{"--foo"},
			Metadata: HostMetadata{
				K0sStatusArgs: []string{"--foo", "--data-dir=/tmp/data", "--token-file=/tmp/token", "--config=/tmp/foo.yaml"},
			},
		}
		_ = h.Connect(context.Background())
		require.False(t, h.FlagsChanged())
		h.InstallFlags = []string{"--bar"}
		require.True(t, h.FlagsChanged())
	})
	t.Run("quoted values", func(t *testing.T) {
		h := Host{
			CompositeConfig: rig.CompositeConfig{Localhost: rig.LocalhostConfig(true)},
			Configurer:      mc,
			DataDir:         "/tmp/data",
			Role:            "controller+worker",
			PrivateAddress:  "10.0.0.1",
			InstallFlags:    []string{"--foo='bar'", "--bar=foo"},
			Metadata: HostMetadata{
				K0sStatusArgs: []string{"--foo=bar", `--bar="foo"`, "--enable-worker=true", "--data-dir=/tmp/data", "--token-file=/tmp/token", "--config=/tmp/foo.yaml", "--kubelet-extra-args=--node-ip=10.0.0.1"},
			},
		}
		_ = h.Connect(context.Background())
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
			CompositeConfig: rig.CompositeConfig{Localhost: rig.LocalhostConfig(true)},
			Configurer:      mc,
			DataDir:         "/tmp/data",
			Role:            "single",
			PrivateAddress:  "10.0.0.1",
			InstallFlags:    []string{"--foo='bar'", `--kubelet-extra-args="--bar=foo --foo='bar'"`},
			Metadata: HostMetadata{
				K0sStatusArgs: []string{"--foo=bar", `--kubelet-extra-args="--bar=foo --foo='bar'"`, "--data-dir=/tmp/data", "--single=true", "--token-file=/tmp/token", "--config=/tmp/foo.yaml"},
			},
		}
		_ = h.Connect(context.Background())
		flags, err := h.K0sInstallFlags()
		require.NoError(t, err)
		require.Equal(t, `--foo=bar --kubelet-extra-args='--bar=foo --foo='"'"'bar'"'"'' --data-dir=/tmp/data --single=true --token-file=/tmp/token --config=/tmp/foo.yaml`, flags.Join(h.FS()))
		require.False(t, h.FlagsChanged())
		h.InstallFlags = []string{"--foo='baz'", `--kubelet-extra-args='--bar=baz --foo="bar"'`}
		flags, err = h.K0sInstallFlags()
		require.NoError(t, err)
		require.Equal(t, `--foo=baz --kubelet-extra-args='--bar=baz --foo="bar"' --data-dir=/tmp/data --single=true --token-file=/tmp/token --config=/tmp/foo.yaml`, flags.Join(h.FS()))
		require.True(t, h.FlagsChanged())
	})
}
