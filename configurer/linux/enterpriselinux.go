package linux

import (
	"fmt"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig/v2/sh"
)

// EnterpriseLinux is a base package for several RHEL-like enterprise linux distributions
type EnterpriseLinux struct {
	configurer.Linux
}

// InstallPackage installs packages via yum
func (l *EnterpriseLinux) InstallPackage(h configurer.Host, pkg ...string) error {
	if err := h.Sudo().Exec(sh.Command("yum", append([]string{"install", "-y"}, pkg...)...)); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	return nil
}
