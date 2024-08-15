package linux

import (
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os/registry"
)

// OpenSUSE provides OS support for OpenSUSE
type OpenSUSE struct {
	SLES
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "opensuse" || os.ID == "opensuse-microos"
		},
		func() interface{} {
			return &OpenSUSE{}
		},
	)
}
