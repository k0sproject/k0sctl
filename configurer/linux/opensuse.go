package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// OpenSUSE provides OS support for OpenSUSE
type OpenSUSE struct {
	SLES
}

var _ configurer.Configurer = (*OpenSUSE)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "opensuse" || os.ID == "opensuse-microos"
		},
		func() any {
			return &OpenSUSE{}
		},
	)
}
