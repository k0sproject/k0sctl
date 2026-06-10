package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// Flatcar provides OS support for Flatcar Container Linux
type Flatcar struct {
	BaseLinux
}

var _ configurer.Configurer = (*Flatcar)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "flatcar"
		},
		func() any {
			fc := &Flatcar{}
			fc.SetPath("K0sBinaryPath", "/opt/bin/k0s")
			return fc
		},
	)
}
