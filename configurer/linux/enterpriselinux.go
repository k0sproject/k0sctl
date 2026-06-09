package linux

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
)

// EnterpriseLinux is a base package for several RHEL-like enterprise linux distributions
type EnterpriseLinux struct {
	configurer.Linux
}

// InstallPackage installs packages via yum
func (l *EnterpriseLinux) InstallPackage(h configurer.Host, pkg ...string) error {
	if err := h.Sudo().Exec(fmt.Sprintf("yum install -y %s", strings.Join(pkg, " "))); err != nil {
		return fmt.Errorf("failed to install packages: %w", err)
	}
	return nil
}
