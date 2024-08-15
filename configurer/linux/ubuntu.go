package linux

import (
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os/registry"
)

// Ubuntu provides OS support for Ubuntu systems
type Ubuntu struct {
	Debian
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "ubuntu"
		},
		func() interface{} {
			return &Ubuntu{}
		},
	)
}
