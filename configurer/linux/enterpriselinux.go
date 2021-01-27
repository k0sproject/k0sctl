package linux

import (
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/linux"
)

// EnterpriseLinux is a base package for several RHEL-like enterprise linux distributions
type EnterpriseLinux struct {
	linux.EnterpriseLinux
}

// InstallKubectl installs kubectl using the gcloud kubernetes repo
func (c EnterpriseLinux) InstallKubectl(h os.Host) error {
	err := c.WriteFile(h, "/etc/yum.repos.d/kubernetes.repo", `[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
`, "0644")

	if err != nil {
		return err
	}

	return c.InstallPackage(h, "kubectl")
}
