package configurer

import (
	"fmt"

	"github.com/k0sproject/rig/os"
)

// Linux is a base module for various linux OS support packages
type Linux struct {
	Host os.Host
}

// NOTE The Linux struct does not embed rig/os.Linux because it will confuse
// go as the distro-configurers' parents embed it too. This means you can't
// add functions to base Linux package that call functions in the rig/os.Linux package,
// you can however write those functions in the distro-configurers.
// An example of this problem is the ReplaceK0sTokenPath function, which would like to
// call `l.ServiceScriptPath("kos")`, which was worked around here by getting the
// path as a parameter.

// Arch returns the host processor architecture in the format k0s expects it
func (l Linux) Arch() (string, error) {
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

// Chmod changes file permissions
func (l Linux) Chmod(path, chmod string) error {
	return l.Host.Execf("sudo chmod %s %s", chmod, path)
}

// K0sCmdf can be used to construct k0s commands in sprintf style.
func (l Linux) K0sCmdf(template string, args ...interface{}) string {
	return fmt.Sprintf("sudo %s %s", l.K0sBinaryPath(), fmt.Sprintf(template, args...))
}

// K0sBinaryPath returns the location of k0s binary
func (l Linux) K0sBinaryPath() string {
	return "/usr/local/bin/k0s"
}

// K0sConfigPath returns the location of k0s configuration file
func (l Linux) K0sConfigPath() string {
	return "/etc/k0s/k0s.yaml"
}

// K0sJoinTokenPath returns the location of k0s join token file
func (l Linux) K0sJoinTokenPath() string {
	return "/etc/k0s/k0stoken"
}

// TempFile returns a temp file path
func (l Linux) TempFile() (string, error) {
	return l.Host.ExecOutput("mktemp")
}

// DownloadK0s performs k0s binary download from github on the host
func (l Linux) DownloadK0s(version, arch string) error {
	tmp, err := l.TempFile()
	if err != nil {
		return err
	}
	defer func() { _ = l.Host.Execf(`rm -f "%s"`, tmp) }()

	url := fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/v%s/k0s-v%s-%s", version, version, arch)
	if err := l.Host.Execf(`curl -sSLf -o "%s" "%s"`, tmp, url); err != nil {
		return err
	}

	return l.Host.Execf(`sudo install -m 0750 -o root -g adm "%s" "%s"`, tmp, l.K0sBinaryPath())
}

// ReplaceK0sTokenPath replaces the config path in the service stub
func (l Linux) ReplaceK0sTokenPath(spath string) error {
	return l.Host.Exec(fmt.Sprintf("sed -i 's^REPLACEME^%s^g' %s", l.K0sJoinTokenPath(), spath))
}

// WebRequestPackage is the name of a package that can be used to perform web requests (curl, ..)
func (l Linux) WebRequestPackage() string {
	return "curl"
}

// FileContains returns true if a file contains the substring
func (l Linux) FileContains(path, s string) bool {
	return l.Host.Execf(`sudo grep -q "%s" "%s"`, s, path) == nil
}

// MoveFile moves a file on the host
func (l Linux) MoveFile(src, dst string) error {
	return l.Host.Execf(`sudo mv "%s" "%s"`, src, dst)
}
