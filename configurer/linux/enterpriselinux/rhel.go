package enterpriselinux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	rigos "github.com/k0sproject/rig/v2/os"
)

// RHEL provides OS support for RedHat Enterprise Linux
type RHEL struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*RHEL)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "rhel" && !strings.Contains(r.Name, "CoreOS")
		},
		func() any {
			return &RHEL{}
		},
	)
}
