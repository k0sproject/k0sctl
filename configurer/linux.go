package configurer

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"al.essio.dev/pkg/shellescape"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
)

// Linux is a base module for various linux OS support packages
type Linux struct {
	paths    map[string]string
	pathMu   sync.RWMutex
	pathOnce sync.Once
}

// OSKind returns the identifier for Linux hosts
func (l *Linux) OSKind() string {
	return "linux"
}

// NOTE The Linux struct does not embed rig/os.Linux because it will confuse
// go as the distro-configurers' parents embed it too. This means you can't
// add functions to base Linux package that call functions in the rig/os.Linux package,
// you can however write those functions in the distro-configurers.
// An example of this problem is the ReplaceK0sTokenPath function, which would like to
// call `l.ServiceScriptPath("kos")`, which was worked around here by getting the
// path as a parameter.

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

// Quote wraps shellescape.Quote for consumers that need OS-aware escaping
func (l *Linux) Quote(value string) string {
	return shellescape.Quote(value)
}

// SetPath sets a path for a key
func (l *Linux) SetPath(key, value string) {
	l.initPaths()
	l.pathMu.Lock()
	defer l.pathMu.Unlock()
	l.paths[key] = value
}

// Arch returns the host processor architecture in the format k0s expects it
func (l *Linux) Arch(h os.Host) (string, error) {
	arch, err := h.ExecOutput("uname -m")
	if err != nil {
		return "", err
	}
	switch arch {
	case "x86_64":
		return "amd64", nil
	case "aarch64":
		return "arm64", nil
	case "armv7l", "armv8l", "aarch32", "arm32", "armhfp", "arm-32":
		return "arm", nil
	default:
		return arch, nil
	}
}

// K0sCmdf can be used to construct k0s commands in sprintf style.
func (l *Linux) K0sCmdf(template string, args ...any) string {
	return fmt.Sprintf("%s %s", l.K0sBinaryPath(), fmt.Sprintf(template, args...))
}

// K0sctlLockFilePath returns a path to a lock file
func (l *Linux) K0sctlLockFilePath(h os.Host) string {
	if h.Exec("test -d /run/lock", exec.Sudo(h)) == nil {
		return "/run/lock/k0sctl"
	}

	return "/tmp/k0sctl.lock"
}

// TempFile returns a temp file path
func (l *Linux) TempFile(h os.Host) (string, error) {
	return h.ExecOutput("mktemp")
}

// TempDir returns a temp dir path
func (l *Linux) TempDir(h os.Host) (string, error) {
	return h.ExecOutput("mktemp -d")
}

var trailingNumberRegex = regexp.MustCompile(`(\d+)$`)

func trailingNumber(s string) (int, bool) {
	match := trailingNumberRegex.FindStringSubmatch(s)
	if len(match) > 0 {
		i, err := strconv.Atoi(match[1])
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

// DownloadURL performs a download from a URL on the host
func (l *Linux) DownloadURL(h os.Host, url, destination string, opts ...exec.Option) error {
	err := h.Exec(fmt.Sprintf(`curl -sSLf -o %s %s`, shellescape.Quote(destination), shellescape.Quote(url)), opts...)
	if err != nil {
		if exitCode, ok := trailingNumber(err.Error()); ok && exitCode == 22 {
			return fmt.Errorf("download failed: http 404 - not found: %w", err)
		}
		return fmt.Errorf("download failed: %w", err)
	}
	return nil
}

// ReplaceK0sTokenPath replaces the config path in the service stub
func (l *Linux) ReplaceK0sTokenPath(h os.Host, spath string) error {
	return h.Exec(fmt.Sprintf("sed -i 's^REPLACEME^%s^g' %s", l.K0sJoinTokenPath(), spath))
}

// FileContains returns true if a file contains the substring
func (l *Linux) FileContains(h os.Host, path, s string) bool {
	return h.Execf(`grep -q "%s" "%s"`, s, path, exec.Sudo(h)) == nil
}

// MoveFile moves a file on the host
func (l *Linux) MoveFile(h os.Host, src, dst string) error {
	return h.Execf(`mv "%s" "%s"`, src, dst, exec.Sudo(h))
}

// Chown sets owner for a file or directory
func (l *Linux) Chown(h os.Host, path, owner string, opts ...exec.Option) error {
	if len(opts) == 0 {
		opts = []exec.Option{exec.Sudo(h)}
	}
	cmd := fmt.Sprintf(`chown %s %s`, shellescape.Quote(owner), shellescape.Quote(path))
	return h.Exec(cmd, opts...)
}

// KubeconfigPath returns the path to a kubeconfig on the host
func (l *Linux) KubeconfigPath(h os.Host, dataDir string) string {
	linux := &os.Linux{}

	// if admin.conf exists, use that
	adminConfPath := path.Join(dataDir, "pki/admin.conf")
	if linux.FileExist(h, adminConfPath) {
		return adminConfPath
	}
	return path.Join(dataDir, "kubelet.conf")
}

// KubectlCmdf returns a command line in sprintf manner for running kubectl on the host using the kubeconfig from KubeconfigPath
func (l *Linux) KubectlCmdf(h os.Host, dataDir, s string, args ...any) string {
	return fmt.Sprintf(`env "KUBECONFIG=%s" %s`, l.KubeconfigPath(h, dataDir), l.K0sCmdf(`kubectl %s`, fmt.Sprintf(s, args...)))
}

// HTTPStatus makes a HTTP GET request to the url and returns the status code or an error
func (l *Linux) HTTPStatus(h os.Host, url string) (int, error) {
	output, err := h.ExecOutput(fmt.Sprintf(`curl -kso /dev/null --connect-timeout 20 -w "%%{http_code}" "%s"`, url))
	if err != nil {
		return -1, err
	}
	status, err := strconv.Atoi(output)
	if err != nil {
		return -1, fmt.Errorf("invalid response: %s", err.Error())
	}

	return status, nil
}

const sbinPath = `PATH=/usr/local/sbin:/usr/sbin:/sbin:$PATH`

// PrivateInterface tries to find a private network interface
func (l *Linux) PrivateInterface(h os.Host) (string, error) {
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
func (l *Linux) PrivateAddress(h os.Host, iface, publicip string) (string, error) {
	output, err := h.ExecOutput(fmt.Sprintf("%s ip -o addr show dev %s scope global", sbinPath, iface))
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

// UpsertFile creates a file in path with content only if the file does not exist already
func (l *Linux) UpsertFile(h os.Host, path, content string) error {
	tmpf, err := l.TempFile(h)
	if err != nil {
		return err
	}
	if err := h.Execf(`cat > "%s"`, tmpf, exec.Stdin(content), exec.Sudo(h)); err != nil {
		return err
	}

	defer func() {
		_ = h.Execf(`rm -f "%s"`, tmpf, exec.Sudo(h))
	}()

	// mv -n is atomic
	if err := h.Execf(`mv -n "%s" "%s"`, tmpf, path, exec.Sudo(h)); err != nil {
		return fmt.Errorf("upsert failed: %w", err)
	}

	// if original tempfile still exists, error out
	if h.Execf(`test -f "%s"`, tmpf) == nil {
		return fmt.Errorf("upsert failed")
	}

	return nil
}

func (l *Linux) DeleteDir(h os.Host, path string, opts ...exec.Option) error {
	return h.Exec(fmt.Sprintf(`rmdir %s`, shellescape.Quote(path)), opts...)
}

func (l *Linux) MachineID(h os.Host) (string, error) {
	return h.ExecOutput(`cat /etc/machine-id || cat /var/lib/dbus/machine-id`)
}

// SystemTime returns the system time as UTC reported by the OS or an error if this fails
func (l *Linux) SystemTime(h os.Host) (time.Time, error) {
	// get utc time as a unix timestamp
	out, err := h.ExecOutput("date -u +\"%s\"")
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get system time: %w", err)
	}
	unixTime, err := strconv.ParseInt(out, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse system time: %w", err)
	}
	return time.Unix(unixTime, 0), nil
}

// LookPath behaves similarly to exec.LookPath but resolves the binary on the remote host
func (l *Linux) LookPath(h os.Host, file string) (string, error) {
	if file == "" {
		return "", fmt.Errorf("invalid binary name")
	}

	output, err := h.ExecOutputf("command -v -- %s", shellescape.Quote(file))
	if err != nil {
		return "", fmt.Errorf("lookpath %s: %w", file, err)
	}

	path := strings.TrimSpace(output)
	if path == "" {
		return "", fmt.Errorf("lookpath %s: not found", file)
	}

	return path, nil
}

// Dir returns the directory part of a path
func (l *Linux) Dir(p string) string {
	return path.Dir(p)
}

// Base returns the base part of a path
func (l *Linux) Base(p string) string {
	return path.Base(p)
}

// HostPath returns the given path unchanged for linux hosts
func (l *Linux) HostPath(p string) string {
	return p
}
