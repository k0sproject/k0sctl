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
			return strings.Contains(os.Name, "CoreOS") && (os.ID == "fedora" || os.ID == "rhel")
		},
		func() interface{} {
			linuxType := &CoreOS{}
			linuxType.PathFuncs = interface{}(linuxType).(configurer.PathFuncs)
			return linuxType
		},
	)
}

func (l CoreOS) InstallPackage(h os.Host, pkg ...string) error {
	for _, p := range pkg {
		if h.Execf("rpm -q %s", p) == nil {
			// already installed
			continue
		}
		if err := h.Execf("sudo rpm-ostree --apply-live --allow-inactive -y install %s", p, exec.Sudo(h)); err != nil {
			return fmt.Errorf("install package %s: %w", p, err)
		}
	}
	return nil
}

func (l CoreOS) Prepare(h os.Host) error {
	if l.SELinuxEnabled(h) {
		return l.InstallPackage(h, "policycoreutils-python-utils")
	}
	return nil
}
