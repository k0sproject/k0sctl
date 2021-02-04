package linux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// Slackware provides OS support for Slackware Linux
type Slackware struct {
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "slackware"
		},
		func() interface{} {
			return Slackware{}
		},
	)
}

// InstallPackage installs packages via slackpkg
func (l Slackware) InstallPackage(h os.Host, pkg ...string) error {
	return h.Execf("sudo slackpkg update && sudo slackpkg install --priority ADD %s", strings.Join(pkg, " "))
}
