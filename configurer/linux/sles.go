package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// SLES provides OS support for SUSE Linux Enterprise Server
type SLES struct {
	BaseLinux
}

var _ configurer.Configurer = (*SLES)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "sles"
		},
		func() any {
			return &SLES{}
		},
	)
}
