package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	rigos "github.com/k0sproject/rig/v2/os"
)

// OracleLinux provides OS support for Oracle Linux
type OracleLinux struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*OracleLinux)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "ol"
		},
		func() any {
			return &OracleLinux{}
		},
	)
}
