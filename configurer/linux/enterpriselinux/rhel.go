package enterpriselinux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// RHEL provides OS support for RedHat Enterprise Linux
type RHEL struct {
	k0slinux.EnterpriseLinux
}

var _ configurer.Configurer = (*RHEL)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "rhel" && !strings.Contains(os.Name, "CoreOS")
		},
		func() any {
			return &RHEL{}
		},
	)
}
