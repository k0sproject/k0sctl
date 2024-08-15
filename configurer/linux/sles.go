package linux

import (
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os"
	"github.com/k0sproject/rig/v2/os/linux"
	"github.com/k0sproject/rig/v2/os/registry"
)

// SLES provides OS support for Suse SUSE Linux Enterprise Server
type SLES struct {
	linux.SLES
	os.Linux
	BaseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "sles"
		},
		func() interface{} {
			return &SLES{}
		},
	)
}
