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

// RHEL provides OS support for RedHat Enterprise Linux
type RHEL struct {
	enterpriselinux.RHEL
	elinux
	k0slinux.EnterpriseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "rhel"
		},
		func(h os.Host) interface{} {
			return &RHEL{
				RHEL: enterpriselinux.RHEL{
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
