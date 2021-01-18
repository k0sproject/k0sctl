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

type elinux linux.EnterpriseLinux

// CentOS provides OS support for CentOS
type CentOS struct {
	enterpriselinux.CentOS
	elinux
	k0slinux.EnterpriseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "centos"
		},
		func(h os.Host) interface{} {
			return &CentOS{
				CentOS: enterpriselinux.CentOS{
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
						Linux: os.Linux{
							Host: h,
						},
					},
				},
			}
		},
	)
}
