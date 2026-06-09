package linux

import (
	"fmt"
	"slices"
	"strings"

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
	if err := h.Sudo().Exec(fmt.Sprintf("pacman -S --noconfirm --noprogressbar %s", strings.Join(pkg, " "))); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	return nil
}
