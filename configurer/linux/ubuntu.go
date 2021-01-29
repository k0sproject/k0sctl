package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/linux"
	"github.com/k0sproject/rig/os/registry"
)

// Ubuntu provides OS support for Ubuntu systems
type Ubuntu struct {
	linux.Ubuntu
	configurer.Linux
}

func init() {
	registry.RegisterOSModule(
		func(os rig.OSVersion) bool {
			return os.ID == "ubuntu"
		},
		func() interface{} {
			return &Ubuntu{}
		},
	)
}

// InstallKubectl installs kubectl using the gcloud kubernetes repo
func (c Ubuntu) InstallKubectl(h os.Host) error {
	if err := c.InstallPackage(h, "apt-transport-https", "gnupg2", "curl"); err != nil {
		return err
	}

	err := h.Exec(`curl -s https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo apt-key add -`)
	if err != nil {
		return err
	}

	err = h.Exec(`echo "deb https://apt.kubernetes.io/ kubernetes-xenial main" | sudo tee -a /etc/apt/sources.list.d/kubernetes.list`)
	if err != nil {
		return err
	}

	return c.InstallPackage(h, "kubectl")
}
