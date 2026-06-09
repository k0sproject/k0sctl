package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
	"github.com/k0sproject/rig/v2/sh"
)

// BaseLinux for tricking go interfaces
type BaseLinux struct {
	configurer.Linux
}

// Alpine provides OS support for Alpine Linux
type Alpine struct {
	BaseLinux
}

var _ configurer.Configurer = (*Alpine)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "alpine"
		},
		func() any {
			return &Alpine{}
		},
	)
}

// InstallPackage installs packages via apk
func (l *Alpine) InstallPackage(h configurer.Host, pkg ...string) error {
	return h.Sudo().Exec(sh.Command("apk", append([]string{"add", "--update"}, pkg...)...))
}

// Prepare installs prerequisite packages on Alpine hosts
func (l *Alpine) Prepare(h configurer.Host) error {
	return l.InstallPackage(h, "findutils", "coreutils")
}
