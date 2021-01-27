package enterpriselinux

import (
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/linux/enterpriselinux"
	"github.com/k0sproject/rig/os/registry"
)

// RHEL provides OS support for RedHat Enterprise Linux
type RHEL struct {
	enterpriselinux.OracleLinux
	k0slinux.EnterpriseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "rhel"
		},
		func() interface{} {
			return RHEL{}
		},
	)
}
