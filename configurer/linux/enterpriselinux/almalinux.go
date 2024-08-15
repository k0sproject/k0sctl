package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os/registry"
)

// AlmaLinux provides OS support for AlmaLinux
type AlmaLinux struct {
	k0slinux.EnterpriseLinux
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "almalinux"
		},
		func() interface{} {
			return &AlmaLinux{}
		},
	)
}
