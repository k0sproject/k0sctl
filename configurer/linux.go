package configurer

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/rig/v2/sh"
)

// Linux is a base module for various linux OS support packages
type Linux struct {
	paths    map[string]string
	pathMu   sync.RWMutex
	pathOnce sync.Once
}

// Kind returns the OS kind identifier for Linux hosts
func (l *Linux) Kind() string {
	return "linux"
}

// OSKind returns the identifier for Linux hosts
func (l *Linux) OSKind() string {
	return "linux"
}

func (l *Linux) initPaths() {
	l.pathOnce.Do(func() {
		l.paths = map[string]string{
			"K0sBinaryPath":      "/usr/local/bin/k0s",
			"K0sConfigPath":      "/etc/k0s/k0s.yaml",
			"K0sJoinTokenPath":   "/etc/k0s/k0stoken",
			"DataDirDefaultPath": "/var/lib/k0s",
		}
	})
}

func (l *Linux) path(key string) string {
	l.initPaths()
	l.pathMu.RLock()
	defer l.pathMu.RUnlock()
	return l.paths[key]
}

// K0sBinaryPath returns the path to the k0s binary on the host
func (l *Linux) K0sBinaryPath() string {
	return l.path("K0sBinaryPath")
}

// K0sConfigPath returns the path to the k0s config file on the host
func (l *Linux) K0sConfigPath() string {
	return l.path("K0sConfigPath")
}

// K0sJoinTokenPath returns the path to the k0s join token file on the host
func (l *Linux) K0sJoinTokenPath() string {
	return l.path("K0sJoinTokenPath")
}

// DataDirDefaultPath returns the path to the k0s data dir on the host
func (l *Linux) DataDirDefaultPath() string {
	return l.path("DataDirDefaultPath")
}

// SetPath sets a path for a key
func (l *Linux) SetPath(key, value string) {
	l.initPaths()
	l.pathMu.Lock()
	defer l.pathMu.Unlock()
	l.paths[key] = value
}

// K0sCmdf can be used to construct k0s commands in sprintf style.
func (l *Linux) K0sCmdf(template string, args ...any) string {
	return fmt.Sprintf("%s %s", l.K0sBinaryPath(), fmt.Sprintf(template, args...))
}

// K0sctlLockFilePath returns a path to a lock file
func (l *Linux) K0sctlLockFilePath(h Host) string {
	if h.Sudo().FS().FileExist("/run/lock") {
		return "/run/lock/k0sctl"
	}

	return "/tmp/k0sctl.lock"
}

// ReplaceK0sTokenPath replaces the config path in the service stub
func (l *Linux) ReplaceK0sTokenPath(h Host, spath string) error {
	return h.Exec(sh.Command("sed", "-i", fmt.Sprintf("s^REPLACEME^%s^g", l.K0sJoinTokenPath()), spath))
}

// KubeconfigPath returns the path to a kubeconfig on the host
func (l *Linux) KubeconfigPath(h Host, dataDir string) string {
	// if admin.conf exists, use that
	adminConfPath := path.Join(dataDir, "pki/admin.conf")
	if h.Sudo().FS().FileExist(adminConfPath) {
		return adminConfPath
	}
	return path.Join(dataDir, "kubelet.conf")
}

// KubectlCmdf returns a command line in sprintf manner for running kubectl on the host using the kubeconfig from KubeconfigPath
func (l *Linux) KubectlCmdf(h Host, dataDir, s string, args ...any) string {
	return fmt.Sprintf(`env "KUBECONFIG=%s" %s`, l.KubeconfigPath(h, dataDir), l.K0sCmdf(`kubectl %s`, fmt.Sprintf(s, args...)))
}

const sbinPath = `PATH=/usr/local/sbin:/usr/sbin:/sbin:$PATH`

// PrivateInterface tries to find a private network interface
func (l *Linux) PrivateInterface(h Host) (string, error) {
	output, err := h.ExecOutput(fmt.Sprintf(`%s; (ip route list scope global | grep -E "\b(172|10|192\.168)\.") || (ip route list | grep -m1 default)`, sbinPath))
	if err == nil {
		re := regexp.MustCompile(`\bdev (\w+)`)
		match := re.FindSubmatch([]byte(output))
		if len(match) > 0 {
			return string(match[1]), nil
		}
		err = fmt.Errorf("can't find 'dev' in output")
	}

	return "", fmt.Errorf("failed to detect a private network interface, define the host privateInterface manually (%s)", err.Error())
}

// PrivateAddress resolves internal ip from private interface
func (l *Linux) PrivateAddress(h Host, iface, publicip string) (string, error) {
	output, err := h.ExecOutput(sbinPath + " " + sh.Command("ip", "-o", "addr", "show", "dev", iface, "scope", "global"))
	if err != nil {
		return "", fmt.Errorf("failed to find private interface with name %s: %s. Make sure you've set correct 'privateInterface' for the host in config", iface, output)
	}

	lines := strings.SplitSeq(output, "\n")
	for line := range lines {
		items := strings.Fields(line)
		if len(items) < 4 {
			continue
		}
		// When subnet mask is 255.255.255.255, CIDR notation is not /32, but it is omitted instead.
		index := strings.Index(items[3], "/")
		addr := items[3]
		if index >= 0 {
			addr = items[3][:index]
		}
		if len(strings.Split(addr, ".")) == 4 {
			if publicip != addr {
				return addr, nil
			}
		}
	}

	return "", fmt.Errorf("not found")
}


// UpdateEnvironment upserts the given key-value pairs into /etc/environment
// (replacing any existing line for the same key) and exports them into the
// current shell environment.
func (l *Linux) UpdateEnvironment(h Host, env map[string]string) error {
	fsys := h.Sudo().FS()
	for k, v := range env {
		patch := remotefs.ReplaceOrAppend(remotefs.ByPrefix(k+"="), fmt.Sprintf("%s=%s", k, v))
		if err := remotefs.PatchFile(fsys, "/etc/environment", []remotefs.Patch{patch}, remotefs.WithCreate(fs.FileMode(0o644))); err != nil {
			return fmt.Errorf("failed to update /etc/environment: %w", err)
		}
	}

	// Export the values into the current session environment.
	if err := h.Sudo().Exec(`while read -r pair; do if [ -n "$pair" ] && [ "${pair#\#}" = "$pair" ]; then export "$pair" || exit 2; fi; done < /etc/environment`); err != nil {
		return fmt.Errorf("failed to update environment: %w", err)
	}
	return nil
}

// FixContainer applies container-specific fixes
func (l *Linux) FixContainer(h Host) error {
	if err := h.Sudo().Exec("mount --make-rshared / 2> /dev/null"); err != nil {
		return fmt.Errorf("failed to mount / as rshared: %w", err)
	}
	return nil
}

// InstallPackage installs packages using the host's package manager. Distro
// configurers override this with the appropriate package manager command.
func (l *Linux) InstallPackage(h Host, pkg ...string) error {
	return fmt.Errorf("package installation is not implemented for this Linux distribution")
}
