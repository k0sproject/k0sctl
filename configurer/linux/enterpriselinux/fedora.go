package enterpriselinux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/os/registry"
)

// Fedora provides OS support for Fedora
type Fedora struct {
	k0slinux.EnterpriseLinux
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "fedora" && !strings.Contains(os.Name, "CoreOS")
		},
		func() interface{} {
			return &Fedora{}
		},
	)
}
