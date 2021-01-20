package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/linux"
	"github.com/k0sproject/rig/os/registry"
)

// SLES provides OS support for Suse SUSE Linux Enterprise Server
type SLES struct {
	linux.SLES
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "sles"
		},
		func(h os.Host) interface{} {
			return &SLES{
				SLES: linux.SLES{
					Linux: os.Linux{
						Host: h,
					},
				},
				Linux: configurer.Linux{
					Host: h,
				},
			}
		},
	)
}
