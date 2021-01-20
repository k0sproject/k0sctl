package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/linux"
	"github.com/k0sproject/rig/os/linux/enterpriselinux"
	"github.com/k0sproject/rig/os/registry"
)

// OracleLinux provides OS support for Oracle Linuc
type OracleLinux struct {
	enterpriselinux.OracleLinux
	elinux
	k0slinux.EnterpriseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "ol"
		},
		func(h os.Host) interface{} {
			return &OracleLinux{
				OracleLinux: enterpriselinux.OracleLinux{
					EnterpriseLinux: linux.EnterpriseLinux{
						Linux: os.Linux{
							Host: h,
						},
					},
				},
				elinux: elinux{
					Linux: os.Linux{
						Host: h,
					},
				},
				EnterpriseLinux: k0slinux.EnterpriseLinux{
					Linux: configurer.Linux{
						Host: h,
					},
				},
			}
		},
	)
}
