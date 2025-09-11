package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// CentOS provides OS support for CentOS
type CentOS struct {
	k0slinux.EnterpriseLinux
	configurer.Linux
}

var _ configurer.Configurer = (*CentOS)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "centos"
		},
		func() any {
			return &CentOS{}
		},
	)
}
