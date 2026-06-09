package linux

import (
	"fmt"

	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
	"github.com/k0sproject/rig/v2/sh"
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

// InstallPackage installs packages via zypper
func (l *SLES) InstallPackage(h configurer.Host, pkg ...string) error {
	if err := h.Sudo().Exec("zypper refresh"); err != nil {
		return fmt.Errorf("failed to refresh zypper: %w", err)
	}
	if err := h.Sudo().Exec(sh.Command("zypper", append([]string{"-n", "install", "-y"}, pkg...)...)); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	return nil
}
