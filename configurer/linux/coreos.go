package linux

import (
	"errors"
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

// InstallPackage is not supported on CoreOS
func (l *CoreOS) InstallPackage(h configurer.Host, pkg ...string) error {
	return errors.New("CoreOS does not support installing packages manually")
}
