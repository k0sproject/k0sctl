package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// RockyLinux provides OS support for RockyLinux
type RockyLinux struct {
	k0slinux.EnterpriseLinux
	configurer.Linux
}

var _ configurer.Configurer = (*RockyLinux)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "rocky"
		},
		func() any {
			return &RockyLinux{}
		},
	)
}
