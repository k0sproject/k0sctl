package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/linux"
	"github.com/k0sproject/rig/os/registry"
)

// Ubuntu provides OS support for Ubuntu systems
type Ubuntu struct {
	linux.Ubuntu
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "ubuntu"
		},
		func(h os.Host) interface{} {
			return &Ubuntu{
				Ubuntu: linux.Ubuntu{
					Linux: os.Linux{
						Host: h,
					},
				},
				Linux: configurer.Linux{
					Linux: os.Linux{
						Host: h,
					},
				},
			}
		},
	)
}
