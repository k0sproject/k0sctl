package linux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// BaseLinux for tricking go interfaces
type BaseLinux struct {
	configurer.Linux
}

// Alpine provides OS support for Alpine Linux
type Alpine struct {
	os.Linux
	BaseLinux
}

var _ configurer.Configurer = (*Alpine)(nil)

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "alpine"
		},
		func() any {
			return &Alpine{}
		},
	)
}

// InstallPackage installs packages via slackpkg
func (l *Alpine) InstallPackage(h os.Host, pkg ...string) error {
	return h.Execf("apk add --update %s", strings.Join(pkg, " "), exec.Sudo(h))
}

func (l *Alpine) Prepare(h os.Host) error {
	return l.InstallPackage(h, "findutils", "coreutils")
}
