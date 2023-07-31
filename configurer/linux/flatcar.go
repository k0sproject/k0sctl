package linux

import (
	"errors"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

type Flatcar struct {
	BaseLinux
	os.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "flatcar"
		},
		func() interface{} {
			linuxType := &Flatcar{}
			linuxType.PathFuncs = interface{}(linuxType).(configurer.PathFuncs)
			return linuxType
		},
	)
}

func (l Flatcar) InstallPackage(h os.Host, pkg ...string) error {
	return errors.New("FlatcarContainerLinux does not support installing packages manually")
}

func (l Flatcar) K0sBinaryPath() string {
	return "/opt/bin/k0s"
}
