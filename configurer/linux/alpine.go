package linux

import (
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
)

// BaseLinux for tricking go interfaces
type BaseLinux struct {
	configurer.Linux
}

var kubectlInstallScript = []string{
	`curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"`,
	`curl -LO "https://dl.k8s.io/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl.sha256"`,
	`echo "$(<kubectl.sha256) kubectl" | sha256sum --check`,
	`sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl`,
}

// InstallKubectl installs kubectl using the curl method
func (l BaseLinux) InstallKubectl(h os.Host) error {
	for _, c := range kubectlInstallScript {
		if err := h.Exec(c); err != nil {
			return err
		}
	}
	return nil
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
			return Alpine{}
		},
	)
}

// InstallPackage installs packages via slackpkg
func (l Alpine) InstallPackage(h os.Host, pkg ...string) error {
	return h.Execf("sudo apk add -U -t k0sctl %s", strings.Join(pkg, " "))
}
