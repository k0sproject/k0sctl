package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// Debian provides OS support for Debian systems
type Debian struct {
	configurer.Linux
}

var _ configurer.Configurer = (*Debian)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "debian"
		},
		func() any {
			return &Debian{}
		},
	)
}
