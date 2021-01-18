package configurer

import (
	"fmt"

	"github.com/k0sproject/rig/os"
)

// Linux is a base module for various linux OS support packages
type Linux struct {
	os.Linux
}

func (l *Linux) CheckPrivilege() error {
	return l.Linux.CheckPrivilege()
}

func (l *Linux) ServiceIsRunning(s string) bool {
	return l.Linux.ServiceIsRunning(s)
}

func (l *Linux) Arch() (string, error) {
	arch, err := l.Host.ExecOutput("uname -m")
	if err != nil {
		return "", err
	}
	switch arch {
	case "x86_64":
		return "amd64", nil
	case "aarch64":
		return "arm64", nil
	default:
		return arch, nil
	}
}

func (l *Linux) K0sCmdf(template string, args ...interface{}) string {
	return fmt.Sprintf("sudo %s %s", l.K0sBinaryPath(), fmt.Sprintf(template, args...))
}

// K0sConfigPath returns location of k0s configuration file
func (l *Linux) K0sBinaryPath() string {
	return "/usr/bin/k0s"
}

// K0sConfigPath returns location of k0s configuration file
func (l *Linux) K0sConfigPath() string {
	return "/etc/k0s/k0s.yaml"
}

// K0sJoinToken returns location of k0s join token file
func (l *Linux) K0sJoinTokenPath() string {
	return "/etc/k0s/k0stoken"
}

// RunK0sDownloader downloads k0s binaries using the online script
func (l *Linux) RunK0sDownloader(version string) error {
	return l.Host.Exec(fmt.Sprintf("curl get.k0s.sh | K0S_VERSION=v%s sh", version))
}
