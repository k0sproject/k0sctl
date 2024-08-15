package enterpriselinux

import (
	"strings"

	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os/registry"
)

// RHEL provides OS support for RedHat Enterprise Linux
type RHEL struct {
	k0slinux.EnterpriseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "rhel" && !strings.Contains(os.Name, "CoreOS")
		},
		func() interface{} {
			return &RHEL{}
		},
	)
}
