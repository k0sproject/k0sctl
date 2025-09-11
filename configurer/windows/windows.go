package windows

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// Windows provides OS support for Windows systems
type Windows struct {
	os.Windows
	configurer.BaseWindows
}

func init() {
	registry.RegisterOSModule(
		func(osv rig.OSVersion) bool {
			return osv.ID == "windows"
		},
		func() interface{} {
			return &Windows{}
		},
	)
}
