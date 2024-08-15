package linux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/exec"
	"github.com/k0sproject/rig/v2/os"
	"github.com/k0sproject/rig/v2/os/registry"
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

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "alpine"
		},
		func() interface{} {
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
