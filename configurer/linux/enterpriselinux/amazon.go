package enterpriselinux

import (
	"github.com/k0sproject/k0sctl/configurer"
	k0slinux "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// AmazonLinux provides OS support for AmazonLinux
type AmazonLinux struct {
	k0slinux.EnterpriseLinux
	configurer.Linux
}

// Hostname on amazon linux will return the full hostname
func (l AmazonLinux) Hostname(h os.Host) string {
	hostname, _ := h.ExecOutput("hostname")

	return hostname
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "amzn"
		},
		func() interface{} {
			linuxType := &AmazonLinux{}
			linuxType.PathFuncs = interface{}(linuxType).(configurer.PathFuncs)
			return linuxType
		},
	)
}
