package configurer

import (
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	ps "github.com/k0sproject/rig/pkg/powershell"
	"github.com/k0sproject/rig/pkg/rigfs"
)

// Windows provides helpers and defaults for Windows hosts
type BaseWindows struct {
	paths    map[string]string
	pathMu   sync.RWMutex
	pathOnce sync.Once
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
func (w *BaseWindows) Arch(h os.Host) (string, error) {
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
func (w *BaseWindows) K0sctlLockFilePath(h os.Host) string {
	// Use a system-wide temp location
	return `C:\\Windows\\Temp\\k0sctl.lock`
}

// TempFile returns a temp file path
func (w *BaseWindows) TempFile(h os.Host) (string, error) {
	output, err := h.ExecOutput(ps.Cmd(`[System.IO.Path]::GetTempFileName()`))
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	output = strings.TrimSpace(output)
	// Normalize to use forward slashes
	output = strings.ReplaceAll(output, "\\", "/")
	return output, nil
}

// TempDir returns a temp dir path
func (w *BaseWindows) TempDir(h os.Host) (string, error) {
	// Create a unique temp directory and output its path
	script := `$p = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString()); New-Item -ItemType Directory -Path $p | Out-Null; Write-Output $p`
	output, err := h.ExecOutput(ps.Cmd(script))
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	output = strings.TrimSpace(output)
	// Normalize to use forward slashes
	output = strings.ReplaceAll(output, "\\", "/")
	return output, nil
}

// DownloadURL performs a download from a URL on the host
func (w *BaseWindows) DownloadURL(h os.Host, url, destination string, opts ...exec.Option) error {
	cmd := ps.Cmd(fmt.Sprintf(`Invoke-WebRequest -UseBasicParsing -Uri %s -OutFile %s`, ps.SingleQuote(url), ps.DoubleQuotePath(ps.ToWindowsPath(destination))))
	if err := h.Exec(cmd, opts...); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	return nil
}

// ReplaceK0sTokenPath replaces the config path in the service stub
func (w *BaseWindows) ReplaceK0sTokenPath(h os.Host, spath string) error {
	// Replace literal REPLACEME with actual token path
	cmd := ps.Cmd(fmt.Sprintf(`(Get-Content -Path %s) -replace 'REPLACEME', %s | Set-Content -Path %s -Encoding ascii`, ps.DoubleQuotePath(ps.ToWindowsPath(spath)), ps.SingleQuote(ps.ToWindowsPath(w.K0sJoinTokenPath())), ps.DoubleQuotePath(ps.ToWindowsPath(spath))))
	return h.Exec(cmd)
}

// FileContains returns true if a file contains the substring
func (w *BaseWindows) FileContains(h os.Host, path, s string) bool {
	cmd := ps.Cmd(fmt.Sprintf(`if (Select-String -Path %s -Pattern %s -SimpleMatch -Quiet) { exit 0 } else { exit 1 }`, ps.DoubleQuotePath(ps.ToWindowsPath(path)), ps.SingleQuote(ps.ToWindowsPath(s))))
	return h.Exec(cmd) == nil
}

// MoveFile moves a file on the host
func (w *BaseWindows) MoveFile(h os.Host, src, dst string) error {
	return h.Exec(ps.Cmd(fmt.Sprintf(`Move-Item -Force -Path %s -Destination %s`, ps.DoubleQuotePath(ps.ToWindowsPath(src)), ps.DoubleQuotePath(ps.ToWindowsPath(dst)))))
}

// Chown is a no-op on Windows; ownership semantics differ and are not managed here
func (w *BaseWindows) Chown(h os.Host, path, owner string, _ ...exec.Option) error {
	return nil
}

type withsudofsys interface {
	SudoFsys() rigfs.Fsys
}

// KubeconfigPath returns the path to a kubeconfig on the host
func (w *BaseWindows) KubeconfigPath(h os.Host, dataDir string) string {
	adminConfPath := path.Join(dataDir, "pki", "admin.conf")
	if hImpl, ok := h.(withsudofsys); ok {
		if _, err := hImpl.SudoFsys().Stat(adminConfPath); err == nil {
			adminConfPath = strings.ReplaceAll(adminConfPath, "\\", "/")
			return adminConfPath
		}
	}
	cfg := path.Join(dataDir, "kubelet.conf")
	cfg = strings.ReplaceAll(cfg, "\\", "/")
	return cfg
}

// KubectlCmdf returns a command line in sprintf manner for running kubectl on the host using the kubeconfig from KubeconfigPath
func (w *BaseWindows) KubectlCmdf(h os.Host, dataDir, s string, args ...interface{}) string {
	if !strings.Contains(s, "--kubeconfig") {
		kubecfgPath := ps.ToWindowsPath(w.KubeconfigPath(h, dataDir))
		s = s + " --kubeconfig=" + ps.DoubleQuotePath(kubecfgPath)
	}

	return w.K0sCmdf(`kubectl %s`, fmt.Sprintf(s, args...))
}

// HTTPStatus makes a HTTP GET request to the url and returns the status code or an error
func (w *BaseWindows) HTTPStatus(h os.Host, url string) (int, error) {
	out, err := h.ExecOutput(ps.Cmd(fmt.Sprintf(`[int][System.Net.WebRequest]::Create(%s).GetResponse().StatusCode`, ps.SingleQuote(url))))
	if err != nil {
		return -1, fmt.Errorf("failed to get HTTP status code: %w", err)
	}
	code, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return -1, fmt.Errorf("invalid response: %w", err)
	}
	return code, nil
}

// PrivateInterface tries to find a private network interface (not implemented for Windows yet)
func (w *BaseWindows) PrivateInterface(h os.Host) (string, error) {
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
	output, err := h.ExecOutput(cmd, exec.Sudo(h))
	if err != nil {
		return "", fmt.Errorf("failed to detect private network interface: %s", err)
	}

	iface := strings.TrimSpace(output)
	if iface == "" {
		return "", fmt.Errorf("no private interface found, define host privateInterface manually")
	}

	return iface, nil
}

// PrivateAddress resolves internal ip from private interface (not implemented for Windows yet)
func (w *BaseWindows) PrivateAddress(h os.Host, iface, publicip string) (string, error) {
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
func (w *BaseWindows) UpsertFile(h os.Host, path, content string) error {
	tmpf, err := w.TempFile(h)
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

func (w *BaseWindows) DeleteDir(h os.Host, path string, opts ...exec.Option) error {
	return h.Exec(ps.Cmd(fmt.Sprintf(`Remove-Item -Recurse -Force -Path %s`, ps.DoubleQuotePath(ps.ToWindowsPath(path)))), opts...)
}

func (w *BaseWindows) MachineID(h os.Host) (string, error) {
	return h.ExecOutput(ps.Cmd(`(Get-ItemProperty -Path 'HKLM:\SOFTWARE\Microsoft\Cryptography' -Name MachineGuid).MachineGuid`))
}

// SystemTime returns the system time as UTC reported by the OS or an error if this fails
func (w *BaseWindows) SystemTime(h os.Host) (time.Time, error) {
	out, err := h.ExecOutput(ps.Cmd(`[DateTimeOffset]::UtcNow.ToUnixTimeSeconds()`))
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get system time: %w", err)
	}
	out = strings.TrimSpace(out)
	var unixTime int64
	if _, scanErr := fmt.Sscanf(out, "%d", &unixTime); scanErr != nil {
		return time.Time{}, fmt.Errorf("failed to parse system time: %v", scanErr)
	}
	return time.Unix(unixTime, 0), nil
}

// LookPath resolves a binary path on the remote Windows host similarly to exec.LookPath
func (w *BaseWindows) LookPath(h os.Host, file string) (string, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return "", fmt.Errorf("invalid binary name")
	}

	script := fmt.Sprintf(`$cmd = Get-Command -Name %s -ErrorAction Stop; if (-not $cmd.Path) { throw 'command path not found' }; Write-Output $cmd.Path`, ps.SingleQuote(file))
	output, err := h.ExecOutput(ps.Cmd(script))
	if err != nil {
		return "", fmt.Errorf("lookpath %s: %w", file, err)
	}

	path := strings.TrimSpace(output)
	if path == "" {
		return "", fmt.Errorf("lookpath %s: not found", file)
	}

	path = strings.ReplaceAll(path, "\\", "/")
	return path, nil
}

// Dir returns the directory part of a path
func (w *BaseWindows) Dir(path string) string {
	index := strings.LastIndexAny(path, `\/`)
	if index == -1 {
		return "."
	}
	return path[:index]
}

// Base returns the last element of a path
func (w *BaseWindows) Base(path string) string {
	index := strings.LastIndexAny(path, `\/`)
	if index == -1 {
		return path
	}
	return path[index+1:]
}

// HostPath converts the provided path to a native Windows path representation
func (w *BaseWindows) HostPath(path string) string {
	return ps.ToWindowsPath(path)
}
