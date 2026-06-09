package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	rigos "github.com/k0sproject/rig/v2/os"
)

// AmazonLinux provides OS support for AmazonLinux
type AmazonLinux struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*AmazonLinux)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "amzn"
		},
		func() any {
			return &AmazonLinux{}
		},
	)
}
