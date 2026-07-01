package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
)

// EnterpriseLinux is a base package for several RHEL-like enterprise linux distributions
type EnterpriseLinux struct {
	configurer.Linux
}
