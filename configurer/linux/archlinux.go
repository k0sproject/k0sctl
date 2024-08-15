package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os/linux"
	"github.com/k0sproject/rig/v2/os/registry"
)

// Archlinux provides OS support for Archlinux systems
type Archlinux struct {
	linux.Archlinux
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "arch" || os.IDLike == "arch"
		},
		func() interface{} {
			return &Archlinux{}
		},
	)
}
