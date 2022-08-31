package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
	"strings"
)

// CoreOS provides OS support for ostree based Fedora & RHEL systems
type CoreOS struct {
	os.Linux
	BaseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "fedora" && strings.Contains(os.Name, "CoreOS") || os.ID == "rhel" && strings.Contains(os.Name, "CoreOS")
		},
		func() interface{} {
			linuxType := &CoreOS{}
			linuxType.PathFuncs = interface{}(linuxType).(configurer.PathFuncs)
			return linuxType
		},
	)
}

func (l CoreOS) InstallPackage(h os.Host, pkg ...string) error {
	return errors.New("CoreOS does not support installing packages manually")
}

func (l CoreOS) K0sBinaryPath() string {
	return "/opt/bin/k0s"
}
