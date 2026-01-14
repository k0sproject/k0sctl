package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/linux"
	"github.com/k0sproject/rig/os/registry"
)

// Archlinux provides OS support for Archlinux systems
type Archlinux struct {
	linux.Archlinux
	configurer.Linux
}

var _ configurer.Configurer = (*Archlinux)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "arch" || os.IDLike == "arch"
		},
		func() any {
			return &Archlinux{}
		},
	)
}
