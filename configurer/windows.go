package configurer

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strconv"
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

// Quote returns a PowerShell-safe double-quoted string when needed
var windowsUnsafePattern = regexp.MustCompile(`[^\w@%+=:,./\\-]`)

func (w *BaseWindows) Quote(value string) string {
	if value == "" {
		return `""`
	}
	if !windowsUnsafePattern.MatchString(value) {
		return value
	}
	return ps.DoubleQuote(value)
}

// SetPath sets a path for a key
func (w *BaseWindows) SetPath(key, value string) {
	w.initPaths()
	w.pathMu.Lock()
	defer w.pathMu.Unlock()
	w.paths[key] = value
}

// Arch returns the host processor architecture in the format k0s expects it
func (w *BaseWindows) Arch(h Host) (string, error) {
	arch, err := h.ExecOutput(ps.Cmd(`$env:PROCESSOR_ARCHITECTURE`))
	if err != nil {
		return "", err
	}

	switch strings.ToUpper(strings.TrimSpace(arch)) {
	case "AMD64", "X86_64":
		return "amd64", nil
	case "ARM64", "AARCH64":
		return "arm64", nil
	case "X86", "386", "I386":
		return "386", nil
	default:
		return strings.ToLower(strings.TrimSpace(arch)), nil
	}
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

// DownloadURL performs a download from a URL on the host
func (w *BaseWindows) DownloadURL(h Host, url, destination string) error {
	cmd := ps.Cmd(fmt.Sprintf(`Invoke-WebRequest -UseBasicParsing -Uri %s -OutFile %s`, ps.SingleQuote(url), ps.DoubleQuotePath(ps.ToWindowsPath(destination))))
	if err := h.Sudo().Exec(cmd); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	return nil
}

// ReplaceK0sTokenPath replaces the config path in the service stub
func (w *BaseWindows) ReplaceK0sTokenPath(h Host, spath string) error {
	// Replace literal REPLACEME with actual token path
	cmd := ps.Cmd(fmt.Sprintf(`(Get-Content -Path %s) -replace 'REPLACEME', %s | Set-Content -Path %s -Encoding ascii`, ps.DoubleQuotePath(ps.ToWindowsPath(spath)), ps.SingleQuote(ps.ToWindowsPath(w.K0sJoinTokenPath())), ps.DoubleQuotePath(ps.ToWindowsPath(spath))))
	return h.Exec(cmd)
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

// UpsertFile creates a file in path with content only if the file does not exist already
func (w *BaseWindows) UpsertFile(h Host, path, content string) error {
	tmpf, err := h.FS().CreateTemp("", "")
	if err != nil {
		return err
	}
	// Write content to temp file
	if err := h.Exec(ps.Cmd(fmt.Sprintf(`Set-Content -Path %s -Value @'
%s
'@ -Encoding ascii`, ps.DoubleQuotePath(ps.ToWindowsPath(tmpf)), content))); err != nil {
		return err
	}

	// Atomically move if destination does not exist
	script := ps.Cmd(fmt.Sprintf(`if (!(Test-Path -Path %s)) { Move-Item -Path %s -Destination %s } else { Remove-Item -Path %s -Force }`, ps.DoubleQuotePath(ps.ToWindowsPath(path)), ps.DoubleQuotePath(ps.ToWindowsPath(tmpf)), ps.DoubleQuotePath(ps.ToWindowsPath(path)), ps.DoubleQuotePath(ps.ToWindowsPath(tmpf))))
	if err := h.Exec(script); err != nil {
		return fmt.Errorf("upsert failed: %w", err)
	}

	// Ensure temp file is gone
	if h.Exec(ps.Cmd(fmt.Sprintf(`Test-Path -Path %s`, ps.DoubleQuotePath(ps.ToWindowsPath(tmpf))))) == nil {
		return fmt.Errorf("upsert failed")
	}
	return nil
}

// HostPath converts the provided path to a native Windows path representation
func (w *BaseWindows) HostPath(path string) string {
	return ps.ToWindowsPath(path)
}

// StartService starts a named service
func (w *BaseWindows) StartService(h Host, name string) error {
	svc, err := h.Sudo().Service(name)
	if err != nil {
		return err
	}
	return svc.Start(context.Background())
}

// StopService stops a named service
func (w *BaseWindows) StopService(h Host, name string) error {
	svc, err := h.Sudo().Service(name)
	if err != nil {
		return err
	}
	return svc.Stop(context.Background())
}

// RestartService restarts a named service
func (w *BaseWindows) RestartService(h Host, name string) error {
	svc, err := h.Sudo().Service(name)
	if err != nil {
		return err
	}
	return svc.Restart(context.Background())
}

// ServiceIsRunning returns true when a named service is running
func (w *BaseWindows) ServiceIsRunning(h Host, name string) bool {
	svc, err := h.Sudo().Service(name)
	if err != nil {
		return false
	}
	return svc.IsRunning(context.Background())
}

// ServiceScriptPath returns an identifier for the Windows service.
func (w *BaseWindows) ServiceScriptPath(h Host, name string) (string, error) {
	svc, err := h.Sudo().Service(name)
	if err != nil {
		return "", err
	}
	return svc.ScriptPath(context.Background())
}

// DaemonReload is a no-op on Windows
func (w *BaseWindows) DaemonReload(h Host) error {
	return nil
}

// UpdateServiceEnvironment is a no-op on Windows; k0s services do not use
// init-system environment overrides there.
func (w *BaseWindows) UpdateServiceEnvironment(h Host, name string, env map[string]string) error {
	return nil
}

// CleanupServiceEnvironment is a no-op on Windows.
func (w *BaseWindows) CleanupServiceEnvironment(h Host, name string) error {
	return nil
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

// WriteFile writes content to a file on the host
func (w *BaseWindows) WriteFile(h Host, filePath, content, perm string) error {
	mode, err := strconv.ParseUint(perm, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid permissions %q: %w", perm, err)
	}
	return h.Sudo().FS().WriteFile(filePath, []byte(content), fs.FileMode(mode))
}

// ReadFile reads a file and returns its contents as a string
func (w *BaseWindows) ReadFile(h Host, filePath string) (string, error) {
	data, err := h.FS().ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// CheckPrivilege verifies the connecting user has administrator privileges
func (w *BaseWindows) CheckPrivilege(h Host) error {
	script := `if (-not ([System.Security.Principal.WindowsPrincipal][System.Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([System.Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'administrator privileges required' }`
	if err := h.Exec(ps.Cmd(script)); err != nil {
		return fmt.Errorf("administrator privileges check failed: %w", err)
	}
	return nil
}

// FixContainer is a no-op on Windows
func (w *BaseWindows) FixContainer(h Host) error {
	return nil
}

// InstallPackage is not supported on Windows
func (w *BaseWindows) InstallPackage(h Host, pkg ...string) error {
	return fmt.Errorf("package installation is not supported on Windows")
}
