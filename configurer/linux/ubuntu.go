package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// Ubuntu provides OS support for Ubuntu systems
type Ubuntu struct {
	Debian
}

var _ configurer.Configurer = (*Ubuntu)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "ubuntu"
		},
		func() any {
			return &Ubuntu{}
		},
	)
}
