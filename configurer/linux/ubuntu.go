package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// Ubuntu provides OS support for Ubuntu systems
type Ubuntu struct {
	Debian
}

var _ configurer.Configurer = (*Ubuntu)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "ubuntu"
		},
		func() any {
			return &Ubuntu{}
		},
	)
}
