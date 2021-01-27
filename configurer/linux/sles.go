package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/linux"
	"github.com/k0sproject/rig/os/registry"
)

// SLES provides OS support for Suse SUSE Linux Enterprise Server
type SLES struct {
	linux.SLES
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "sles"
		},
		func() interface{} {
			return SLES{}
		},
	)
}

var kubectlInstallScript = []string{
	`curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"`,
	`curl -LO "https://dl.k8s.io/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl.sha256"`,
	`echo "$(<kubectl.sha256) kubectl" | sha256sum --check`,
	`sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl`,
}

// InstallKubectl installs kubectl using the curl method
func (l SLES) InstallKubectl(h os.Host) error {
	for _, c := range kubectlInstallScript {
		if err := h.Exec(c); err != nil {
			return err
		}
	}
	return nil
}
