package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// OpenSUSE provides OS support for OpenSUSE
type OpenSUSE struct {
	SLES
}

var _ configurer.Configurer = (*OpenSUSE)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "opensuse" || r.ID == "opensuse-microos"
		},
		func() any {
			return &OpenSUSE{}
		},
	)
}
