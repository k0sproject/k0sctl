package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os/linux"
	"github.com/k0sproject/rig/v2/os/registry"
)

// Debian provides OS support for Debian systems
type Debian struct {
	linux.Ubuntu
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "debian"
		},
		func() interface{} {
			return &Debian{}
		},
	)
}
