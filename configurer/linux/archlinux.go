package linux

import (
	"context"
	"slices"

	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// Archlinux provides OS support for Archlinux systems
type Archlinux struct {
	configurer.Linux
}

var _ configurer.Configurer = (*Archlinux)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "arch" || slices.Contains(r.IDLike, "arch")
		},
		func() any {
			return &Archlinux{}
		},
	)
}

// InstallPackage installs packages via pacman
func (l *Archlinux) InstallPackage(h configurer.Host, pkg ...string) error {
	return h.Sudo().PackageManager().Install(context.Background(), pkg...)
}
