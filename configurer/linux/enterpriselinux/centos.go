package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	rigos "github.com/k0sproject/rig/v2/os"
)

// CentOS provides OS support for CentOS
type CentOS struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*CentOS)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "centos"
		},
		func() any {
			return &CentOS{}
		},
	)
}
