package configurer

import (
	"fmt"
	"path"
	"strings"
	"sync"

	ps "github.com/k0sproject/rig/v2/powershell"
)

// BaseWindows provides helpers and defaults for Windows hosts
type BaseWindows struct {
	paths    map[string]string
	pathMu   sync.RWMutex
	pathOnce sync.Once
}

// Kind returns the OS kind identifier for Windows hosts
func (w *BaseWindows) Kind() string {
	return "windows"
}

// OSKind returns the identifier for Windows hosts
func (w *BaseWindows) OSKind() string {
	return "windows"
}

func (w *BaseWindows) initPaths() {
	w.pathOnce.Do(func() {
		w.paths = map[string]string{
			"K0sBinaryPath":      `C:/Program Files/k0s/k0s.exe`,
			"K0sConfigPath":      `C:/etc/k0s/k0s.yaml`,
			"K0sJoinTokenPath":   `C:/etc/k0s/k0stoken`,
			"DataDirDefaultPath": `C:/var/lib/k0s`,
		}
	})
}

func (w *BaseWindows) path(key string) string {
	w.initPaths()
	w.pathMu.RLock()
	defer w.pathMu.RUnlock()
	return w.paths[key]
}

// K0sBinaryPath returns the path to the k0s binary on the host
func (w *BaseWindows) K0sBinaryPath() string {
	return w.path("K0sBinaryPath")
}

// K0sConfigPath returns the path to the k0s config file on the host
func (w *BaseWindows) K0sConfigPath() string {
	return w.path("K0sConfigPath")
}

// K0sJoinTokenPath returns the path to the k0s join token file on the host
func (w *BaseWindows) K0sJoinTokenPath() string {
	return w.path("K0sJoinTokenPath")
}

// DataDirDefaultPath returns the path to the k0s data dir on the host
func (w *BaseWindows) DataDirDefaultPath() string {
	return w.path("DataDirDefaultPath")
}

// SetPath sets a path for a key
func (w *BaseWindows) SetPath(key, value string) {
	w.initPaths()
	w.pathMu.Lock()
	defer w.pathMu.Unlock()
	w.paths[key] = value
}

// K0sCmdf can be used to construct k0s commands in sprintf style.
func (w *BaseWindows) K0sCmdf(template string, args ...interface{}) string {
	return ps.Cmd(fmt.Sprintf("& %s %s",
		ps.DoubleQuotePath(ps.ToWindowsPath(w.K0sBinaryPath())),
		fmt.Sprintf(template, args...),
	))
}

// K0sctlLockFilePath returns a path to a lock file
func (w *BaseWindows) K0sctlLockFilePath(h Host) string {
	// Use a system-wide temp location
	return `C:\\Windows\\Temp\\k0sctl.lock`
}

// KubeconfigPath returns the path to a kubeconfig on the host
func (w *BaseWindows) KubeconfigPath(h Host, dataDir string) string {
	adminConfPath := path.Join(dataDir, "pki", "admin.conf")
	if h.Sudo().FS().FileExist(adminConfPath) {
		return strings.ReplaceAll(adminConfPath, "\\", "/")
	}
	return strings.ReplaceAll(path.Join(dataDir, "kubelet.conf"), "\\", "/")
}

// KubectlCmdf returns a command line in sprintf manner for running kubectl on the host using the kubeconfig from KubeconfigPath
func (w *BaseWindows) KubectlCmdf(h Host, dataDir, s string, args ...interface{}) string {
	if !strings.Contains(s, "--kubeconfig") {
		kubecfgPath := ps.ToWindowsPath(w.KubeconfigPath(h, dataDir))
		s = s + " --kubeconfig=" + ps.DoubleQuotePath(kubecfgPath)
	}

	return w.K0sCmdf(`kubectl %s`, fmt.Sprintf(s, args...))
}

// PrivateInterface tries to find a private network interface
func (w *BaseWindows) PrivateInterface(h Host) (string, error) {
	cmd := ps.Cmd(`
$if = Get-NetIPAddress -AddressFamily IPv4 |
  Where-Object { $_.IPAddress -match '^(10\.|172\.(1[6-9]|2[0-9]|3[0-1])\.|192\.168\.)' } |
  Sort-Object InterfaceMetric |
  Select-Object -First 1 -ExpandProperty InterfaceAlias;
if (-not $if) {
  $if = Get-NetRoute -DestinationPrefix '0.0.0.0/0' |
    Sort-Object RouteMetric, InterfaceMetric |
    Select-Object -First 1 -ExpandProperty InterfaceAlias
};
$if`)
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	cmd = strings.ReplaceAll(cmd, "\t", " ")
	output, err := h.ExecOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to detect private network interface: %s", err)
	}

	iface := strings.TrimSpace(output)
	if iface == "" {
		return "", fmt.Errorf("no private interface found, define host privateInterface manually")
	}

	return iface, nil
}

// PrivateAddress resolves internal ip from private interface
func (w *BaseWindows) PrivateAddress(h Host, iface, publicip string) (string, error) {
	cmd := ps.Cmd(fmt.Sprintf(`
(Get-NetIPAddress -InterfaceAlias %s -AddressFamily IPv4 |
  Where-Object {
    $_.AddressState -eq 'Preferred' -and
    $_.IPAddress -match '^(10\.|172\.(1[6-9]|2[0-9]|3[0-1])\.|192\.168\.)' -and
    $_.IPAddress -ne '%s'
  } |
  Select-Object -First 1 -ExpandProperty IPAddress)
`, ps.SingleQuote(iface), publicip))
	cmd = strings.ReplaceAll(cmd, "\n", " ")
	cmd = strings.ReplaceAll(cmd, "\t", " ")
	output, err := h.ExecOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("failed to get IP for interface %s: %s", iface, err)
	}

	ip := strings.TrimSpace(output)
	if ip == "" {
		return "", fmt.Errorf("no IPv4 address found for interface %s", iface)
	}

	if ip == publicip {
		return "", fmt.Errorf("resolved IP equals public IP, no private address found")
	}

	return ip, nil
}


// UpdateEnvironment sets machine-level environment variables on the host
func (w *BaseWindows) UpdateEnvironment(h Host, env map[string]string) error {
	for k, v := range env {
		script := fmt.Sprintf(`[System.Environment]::SetEnvironmentVariable(%s, %s, [System.EnvironmentVariableTarget]::Machine)`, ps.SingleQuote(k), ps.SingleQuote(v))
		if err := h.Exec(ps.Cmd(script)); err != nil {
			return err
		}
	}
	return nil
}

// FixContainer is a no-op on Windows
func (w *BaseWindows) FixContainer(h Host) error {
	return nil
}

