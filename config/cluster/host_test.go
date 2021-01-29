package cluster

import (
	"fmt"
	"testing"

	cfg "github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/stretchr/testify/require"
)

func TestHostK0sServiceName(t *testing.T) {
	h := Host{Role: "worker"}
	require.Equal(t, "k0sworker", h.K0sServiceName())
	h.Role = "server"
	require.Equal(t, "k0sserver", h.K0sServiceName())
	h.Role = "server+worker"
	require.Equal(t, "k0sserver", h.K0sServiceName())
}

type mockconfigurer struct {
	cfg.Linux
	linux.Ubuntu
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

func TestK0sInstallCommand(t *testing.T) {
	h := Host{Role: "worker"}
	h.Configurer = &mockconfigurer{}

	require.Equal(t, `k0s install worker --token-file "from-configurer" --config "from-configurer"`, h.K0sInstallCommand())

	h.Role = "server"
	h.Metadata.IsK0sLeader = true
	require.Equal(t, `k0s install server --config "from-configurer"`, h.K0sInstallCommand())
	h.Metadata.IsK0sLeader = false
	require.Equal(t, `k0s install server --token-file "from-configurer" --config "from-configurer"`, h.K0sInstallCommand())

	h.Role = "server+worker"
	h.Metadata.IsK0sLeader = true
	require.Equal(t, `k0s install server --enable-worker --config "from-configurer"`, h.K0sInstallCommand())
	h.Metadata.IsK0sLeader = false
	require.Equal(t, `k0s install server --enable-worker --token-file "from-configurer" --config "from-configurer"`, h.K0sInstallCommand())
}
