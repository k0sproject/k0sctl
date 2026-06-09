package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	rigos "github.com/k0sproject/rig/v2/os"
)

// RockyLinux provides OS support for RockyLinux
type RockyLinux struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*RockyLinux)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "rocky"
		},
		func() any {
			return &RockyLinux{}
		},
	)
}
