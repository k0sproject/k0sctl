package linux

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// CoreOS provides OS support for ostree based Fedora & RHEL systems
type CoreOS struct {
	os.Linux
	BaseLinux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return strings.Contains(os.Version, "CoreOS") && (os.ID == "fedora" || os.ID == "rhel")
		},
		func() interface{} {
			linuxType := &CoreOS{}
			linuxType.PathFuncs = interface{}(linuxType).(configurer.PathFuncs)
			return linuxType
		},
	)
}

func (l CoreOS) InstallPackage(h os.Host, pkg ...string) error {
	if err := h.Execf("sudo rpm-ostree --apply-live --allow-inactive -y install %s", strings.Join(pkg, " "), exec.Sudo(h)); err != nil {
		return fmt.Errorf("install packages: %w", err)
	}
	return nil
}

func (l CoreOS) Prepare(h os.Host) error {
	return l.InstallPackage(h, "policycoreutils-python-utils")
}

func (l CoreOS) K0sBinaryPath() string {
	return "/opt/bin/k0s"
}
