package linux

import (
	"context"

	"github.com/k0sproject/k0sctl/configurer"
)

// EnterpriseLinux is a base package for several RHEL-like enterprise linux distributions
type EnterpriseLinux struct {
	configurer.Linux
}

// InstallPackage installs packages via yum or dnf
func (l *EnterpriseLinux) InstallPackage(h configurer.Host, pkg ...string) error {
	return h.Sudo().PackageManager().Install(context.Background(), pkg...)
}
