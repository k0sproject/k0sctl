package configurer

import (
	"fmt"
	"strconv"

	"github.com/k0sproject/rig/os"
)

// Linux is a base module for various linux OS support packages
type Linux struct{}

// NOTE The Linux struct does not embed rig/os.Linux because it will confuse
// go as the distro-configurers' parents embed it too. This means you can't
// add functions to base Linux package that call functions in the rig/os.Linux package,
// you can however write those functions in the distro-configurers.
// An example of this problem is the ReplaceK0sTokenPath function, which would like to
// call `l.ServiceScriptPath("kos")`, which was worked around here by getting the
// path as a parameter.

// Arch returns the host processor architecture in the format k0s expects it
func (l Linux) Arch(h os.Host) (string, error) {
	arch, err := h.ExecOutput("uname -m")
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
func (l Linux) Chmod(h os.Host, path, chmod string) error {
	return h.Execf("sudo chmod %s %s", chmod, path)
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
func (l Linux) TempFile(h os.Host) (string, error) {
	return h.ExecOutput("mktemp")
}

// DownloadK0s performs k0s binary download from github on the host
func (l Linux) DownloadK0s(h os.Host, version, arch string) error {
	tmp, err := l.TempFile(h)
	if err != nil {
		return err
	}
	defer func() { _ = h.Execf(`rm -f "%s"`, tmp) }()

	url := fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/v%s/k0s-v%s-%s", version, version, arch)
	if err := h.Execf(`curl -sSLf -o "%s" "%s"`, tmp, url); err != nil {
		return err
	}

	return h.Execf(`sudo install -m 0750 -o root -g adm "%s" "%s"`, tmp, l.K0sBinaryPath())
}

// ReplaceK0sTokenPath replaces the config path in the service stub
func (l Linux) ReplaceK0sTokenPath(h os.Host, spath string) error {
	return h.Exec(fmt.Sprintf("sed -i 's^REPLACEME^%s^g' %s", l.K0sJoinTokenPath(), spath))
}

// WebRequestPackage is the name of a package that can be used to perform web requests (curl, ..)
func (l Linux) WebRequestPackage() string {
	return "curl"
}

// FileContains returns true if a file contains the substring
func (l Linux) FileContains(h os.Host, path, s string) bool {
	return h.Execf(`sudo grep -q "%s" "%s"`, s, path) == nil
}

// MoveFile moves a file on the host
func (l Linux) MoveFile(h os.Host, src, dst string) error {
	return h.Execf(`sudo mv "%s" "%s"`, src, dst)
}

// KubeconfigPath returns the path to a kubeconfig on the host
func (l Linux) KubeconfigPath() string {
	return "/var/lib/k0s/pki/admin.conf"
}

// KubectlCmdf returns a command line in sprintf manner for running kubectl on the host using the kubeconfig from KubeconfigPath
func (l Linux) KubectlCmdf(s string, args ...interface{}) string {
	return fmt.Sprintf(`sudo kubectl --kubeconfig "%s" %s`, l.KubeconfigPath(), fmt.Sprintf(s, args...))
}

// HTTPStatus makes a HTTP GET request to the url and returns the status code or an error
func (l Linux) HTTPStatus(h os.Host, url string) (int, error) {
	output, err := h.ExecOutput(fmt.Sprintf(`curl -kso /dev/null -w "%%{http_code}" "%s"`, url))
	if err != nil {
		return -1, err
	}
	status, err := strconv.Atoi(output)
	if err != nil {
		return -1, fmt.Errorf("invalid response: %s", err.Error())
	}

	return status, nil
}
