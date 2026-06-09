package linux

import (
	"fmt"
	"strings"

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

// InstallPackage installs packages via apt-get
func (l *Debian) InstallPackage(h configurer.Host, pkg ...string) error {
	if err := h.Sudo().Exec("apt-get update"); err != nil {
		return fmt.Errorf("failed to update apt cache: %w", err)
	}
	if err := h.Sudo().Exec(fmt.Sprintf("DEBIAN_FRONTEND=noninteractive apt-get install -y -q %s", strings.Join(pkg, " "))); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	return nil
}
