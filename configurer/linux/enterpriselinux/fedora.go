package enterpriselinux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os/registry"
)

// Fedora provides OS support for Fedora
type Fedora struct {
	k0slinux.EnterpriseLinux
	configurer.Linux
}

var _ configurer.Configurer = (*Fedora)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "fedora" && !strings.Contains(os.Name, "CoreOS")
		},
		func() any {
			return &Fedora{}
		},
	)
}
