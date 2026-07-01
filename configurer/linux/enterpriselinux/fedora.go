package enterpriselinux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	rigos "github.com/k0sproject/rig/v2/os"
)

// Fedora provides OS support for Fedora
type Fedora struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*Fedora)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "fedora" && !strings.Contains(r.Name, "CoreOS")
		},
		func() any {
			return &Fedora{}
		},
	)
}
