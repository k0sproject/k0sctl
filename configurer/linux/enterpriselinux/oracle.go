package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// OracleLinux provides OS support for Oracle Linuc
type OracleLinux struct {
	k0slinux.EnterpriseLinux
	configurer.Linux
}

var _ configurer.Configurer = (*OracleLinux)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "ol"
		},
		func() any {
			return &OracleLinux{}
		},
	)
}
