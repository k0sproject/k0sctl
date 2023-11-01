package linux

import (
	"errors"
	"strings"

	"github.com/k0sproject/rig"
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
			return &CoreOS{}
		},
	)
}

func (l *CoreOS) InstallPackage(h os.Host, pkg ...string) error {
	return errors.New("CoreOS does not support installing packages manually")
}
