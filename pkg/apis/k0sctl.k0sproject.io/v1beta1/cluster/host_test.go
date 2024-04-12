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
}

func (c *mockconfigurer) Chmod(_ os.Host, _, _ string, _ ...exec.Option) error {
	return nil
}

func (c *mockconfigurer) MkDir(_ os.Host, _ string, _ ...exec.Option) error {
	return nil
}

func (c *mockconfigurer) K0sCmdf(s string, args ...interface{}) string {
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

func TestUnQE(t *testing.T) {
	require.Equal(t, `hello`, unQE(`hello`))
	require.Equal(t, `hello`, unQE(`"hello"`))
	require.Equal(t, `hello "world"`, unQE(`"hello \"world\""`))
}

func TestK0sInstallCommand(t *testing.T) {
	h := Host{Role: "worker", DataDir: "/tmp/k0s", Connection: rig.Connection{Localhost: &rig.Localhost{Enabled: true}}}
	_ = h.Connect()
	h.Configurer = &mockconfigurer{}
	h.Configurer.SetPath("K0sConfigPath", "from-configurer")
	h.Configurer.SetPath("K0sJoinTokenPath", "from-configurer")

	cmd, err := h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --data-dir=/tmp/k0s --token-file "from-configurer"`, cmd)

	h.Role = "controller"
	h.Metadata.IsK0sLeader = true
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --config "from-configurer"`, cmd)

	h.Metadata.IsK0sLeader = false
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --token-file "from-configurer" --config "from-configurer"`, cmd)

	h.Role = "controller+worker"
	h.Metadata.IsK0sLeader = true
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --enable-worker --config "from-configurer"`, cmd)

	h.Metadata.IsK0sLeader = false
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install controller --data-dir=/tmp/k0s --enable-worker --token-file "from-configurer" --config "from-configurer"`, cmd)

	h.Role = "worker"
	h.PrivateAddress = "10.0.0.9"
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --data-dir=/tmp/k0s --token-file "from-configurer" --kubelet-extra-args="--node-ip=10.0.0.9"`, cmd)

	h.InstallFlags = []string{`--kubelet-extra-args="--foo bar"`}
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --kubelet-extra-args="--foo bar --node-ip=10.0.0.9" --data-dir=/tmp/k0s --token-file "from-configurer"`, cmd)

	h.InstallFlags = []string{`--enable-cloud-provider`}
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `k0s install worker --enable-cloud-provider --data-dir=/tmp/k0s --token-file "from-configurer"`, cmd)
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
