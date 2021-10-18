package linux

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// Slackware provides OS support for Slackware Linux
type Slackware struct {
	BaseLinux
	os.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "slackware"
		},
		func() interface{} {
			linuxType := &Slackware{}
			linuxType.PathFuncs = interface{}(linuxType).(configurer.PathFuncs)
			return linuxType
		},
	)
}

// InstallPackage installs packages via slackpkg
func (l Slackware) InstallPackage(h os.Host, pkg ...string) error {
	updatecmd, err := h.Sudo("slackpkg update")
	if err != nil {
		return err
	}
	installcmd, err := h.Sudo(fmt.Sprintf("slackpkg install --priority ADD %s", strings.Join(pkg, " ")))
	if err != nil {
		return err
	}

	return h.Execf("%s && %s", updatecmd, installcmd)
}
