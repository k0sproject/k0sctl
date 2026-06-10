package linux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// CoreOS provides OS support for ostree based Fedora & RHEL systems
type CoreOS struct {
	BaseLinux
}

var _ configurer.Configurer = (*CoreOS)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return strings.Contains(r.Name, "CoreOS") && (r.ID == "fedora" || r.ID == "rhel")
		},
		func() any {
			return &CoreOS{}
		},
	)
}
