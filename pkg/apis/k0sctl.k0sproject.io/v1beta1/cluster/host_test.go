package cluster

import (
	"fmt"
	"testing"

	cfg "github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
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

func (c mockconfigurer) Arch(_ os.Host) (string, error) {
	return "mockarch", nil
}

func (c mockconfigurer) Chmod(_ os.Host, _, _ string, _ ...exec.Option) error {
	return nil
}

func (c mockconfigurer) MkDir(_ os.Host, _ string, _ ...exec.Option) error {
	return nil
}

func (c mockconfigurer) K0sJoinTokenPath() string {
	return "from-configurer"
}

func (c mockconfigurer) K0sConfigPath() string {
	return "from-configurer"
}

func (c mockconfigurer) K0sCmdf(s string, args ...interface{}) string {
	return fmt.Sprintf("k0s %s", fmt.Sprintf(s, args...))
}

func TestK0sJoinTokenPath(t *testing.T) {
	h := Host{}
	h.Configurer = &mockconfigurer{}

	require.Equal(t, "from-configurer", h.K0sJoinTokenPath())

	h.InstallFlags.Add("--token-file from-install-flags")
	require.Equal(t, "from-install-flags", h.K0sJoinTokenPath())
}

func TestK0sConfigPath(t *testing.T) {
	h := Host{}
	h.Configurer = &mockconfigurer{}

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
	h := Host{Role: "worker", DataDir: "/tmp/k0s"}
	h.Configurer = &mockconfigurer{}

	cmd, err := h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install worker --token-file "from-configurer"`, cmd)

	h.Role = "controller"
	h.Metadata.IsK0sLeader = true
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install controller --config "from-configurer"`, cmd)

	h.Metadata.IsK0sLeader = false
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install controller --token-file "from-configurer" --config "from-configurer"`, cmd)

	h.Role = "controller+worker"
	h.Metadata.IsK0sLeader = true
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install controller --enable-worker --config "from-configurer"`, cmd)
	h.Metadata.IsK0sLeader = false
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install controller --enable-worker --token-file "from-configurer" --config "from-configurer"`, cmd)

	h.Role = "worker"
	h.PrivateAddress = "10.0.0.9"
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install worker --token-file "from-configurer" --kubelet-extra-args="--node-ip=10.0.0.9"`, cmd)

	h.InstallFlags = []string{`--kubelet-extra-args="--foo bar"`}
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install worker --kubelet-extra-args="--foo bar --node-ip=10.0.0.9" --token-file "from-configurer"`, cmd)

	h.InstallFlags = []string{`--enable-cloud-provider`}
	cmd, err = h.K0sInstallCommand()
	require.NoError(t, err)
	require.Equal(t, `/usr/local/bin/k0s --data-dir=/tmp/k0s install worker --enable-cloud-provider --token-file "from-configurer"`, cmd)
}
