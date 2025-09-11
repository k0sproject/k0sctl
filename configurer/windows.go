package configurer

import (
	"bufio"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	ps "github.com/k0sproject/rig/pkg/powershell"
	"github.com/k0sproject/version"
)

// Windows provides helpers and defaults for Windows hosts
type BaseWindows struct {
	paths  map[string]string
	pathMu sync.Mutex
}

func (w *BaseWindows) initPaths() {
	if w.paths != nil {
		return
	}
	w.paths = map[string]string{
		"K0sBinaryPath":      `C:\\Program Files\\k0s\\k0s.exe`,
		"K0sConfigPath":      `C:\\ProgramData\\k0s\\k0s.yaml`,
		"K0sJoinTokenPath":   `C:\\ProgramData\\k0s\\k0stoken`,
		"DataDirDefaultPath": `C:\\ProgramData\\k0s`,
	}
}

// K0sBinaryPath returns the path to the k0s binary on the host
func (w *BaseWindows) K0sBinaryPath() string {
	w.pathMu.Lock()
	defer w.pathMu.Unlock()
	w.initPaths()
	return w.paths["K0sBinaryPath"]
}

// K0sConfigPath returns the path to the k0s config file on the host
func (w *BaseWindows) K0sConfigPath() string {
	w.pathMu.Lock()
	defer w.pathMu.Unlock()
	w.initPaths()
	return w.paths["K0sConfigPath"]
}

// K0sJoinTokenPath returns the path to the k0s join token file on the host
func (w *BaseWindows) K0sJoinTokenPath() string {
	w.pathMu.Lock()
	defer w.pathMu.Unlock()
	w.initPaths()
	return w.paths["K0sJoinTokenPath"]
}

// DataDirDefaultPath returns the path to the k0s data dir on the host
func (w *BaseWindows) DataDirDefaultPath() string {
	w.pathMu.Lock()
	defer w.pathMu.Unlock()
	w.initPaths()
	return w.paths["DataDirDefaultPath"]
}

// SetPath sets a path for a key
func (w *BaseWindows) SetPath(key, value string) {
	w.pathMu.Lock()
	defer w.pathMu.Unlock()
	w.initPaths()
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
	return fmt.Sprintf(`& %s %s`, ps.DoubleQuotePath(w.K0sBinaryPath()), fmt.Sprintf(template, args...))
}

func (w *BaseWindows) K0sBinaryVersion(h os.Host) (*version.Version, error) {
	k0sVersionCmd := w.K0sCmdf("version")
	output, err := h.ExecOutput(k0sVersionCmd)
	if err != nil {
		return nil, err
	}

	ver, err := version.NewVersion(strings.TrimSpace(output))
	if err != nil {
		return nil, err
	}
	return ver, nil
}

// K0sctlLockFilePath returns a path to a lock file
func (w *BaseWindows) K0sctlLockFilePath(h os.Host) string {
	// Use a system-wide temp location
	return `C:\\Windows\\Temp\\k0sctl.lock`
}

// TempFile returns a temp file path
func (w *BaseWindows) TempFile(h os.Host) (string, error) {
	// Use .NET to generate a temp file path
	return h.ExecOutput(ps.Cmd(`[System.IO.Path]::GetTempFileName()`))
}

// TempDir returns a temp dir path
func (w *BaseWindows) TempDir(h os.Host) (string, error) {
	// Create a unique temp directory and output its path
	script := `$p = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid().ToString()); New-Item -ItemType Directory -Path $p | Out-Null; Write-Output $p`
	return h.ExecOutput(ps.Cmd(script))
}

// DownloadURL performs a download from a URL on the host
func (w *BaseWindows) DownloadURL(h os.Host, url, destination string, opts ...exec.Option) error {
	cmd := ps.Cmd(fmt.Sprintf(`Invoke-WebRequest -UseBasicParsing -Uri %s -OutFile %s`, ps.SingleQuote(url), ps.DoubleQuotePath(destination)))
	if err := h.Exec(cmd, opts...); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	return nil
}

// DownloadK0s performs k0s binary download from github on the host
func (w *BaseWindows) DownloadK0s(h os.Host, path string, versionV *version.Version, arch string, opts ...exec.Option) error {
	v := strings.ReplaceAll(strings.TrimPrefix(versionV.String(), "v"), "+", "%2B")
	url := fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/v%[1]s/k0s-v%[1]s-%[2]s.exe", v, arch)
	if err := w.DownloadURL(h, url, path, opts...); err != nil {
		return fmt.Errorf("failed to download k0s - check connectivity and k0s version validity: %w", err)
	}
	return nil
}

// ReplaceK0sTokenPath replaces the config path in the service stub
func (w *BaseWindows) ReplaceK0sTokenPath(h os.Host, spath string) error {
	// Replace literal REPLACEME with actual token path
	cmd := ps.Cmd(fmt.Sprintf(`(Get-Content -Path %s) -replace 'REPLACEME', %s | Set-Content -Path %s -Encoding ascii`, ps.DoubleQuotePath(spath), ps.SingleQuote(w.K0sJoinTokenPath()), ps.DoubleQuotePath(spath)))
	return h.Exec(cmd)
}

// FileContains returns true if a file contains the substring
func (w *BaseWindows) FileContains(h os.Host, path, s string) bool {
	cmd := ps.Cmd(fmt.Sprintf(`if (Select-String -Path %s -Pattern %s -SimpleMatch -Quiet) { exit 0 } else { exit 1 }`, ps.DoubleQuotePath(path), ps.SingleQuote(s)))
	return h.Exec(cmd) == nil
}

// MoveFile moves a file on the host
func (w *BaseWindows) MoveFile(h os.Host, src, dst string) error {
	return h.Exec(ps.Cmd(fmt.Sprintf(`Move-Item -Force -Path %s -Destination %s`, ps.DoubleQuotePath(src), ps.DoubleQuotePath(dst))))
}

// KubeconfigPath returns the path to a kubeconfig on the host
func (w *BaseWindows) KubeconfigPath(h os.Host, dataDir string) string {
	win := &os.Windows{}
	adminConfPath := filepath.Join(dataDir, "pki", "admin.conf")
	if win.FileExist(h, adminConfPath) {
		return adminConfPath
	}
	return filepath.Join(dataDir, "kubelet.conf")
}

// KubectlCmdf returns a command line in sprintf manner for running kubectl on the host using the kubeconfig from KubeconfigPath
func (w *BaseWindows) KubectlCmdf(h os.Host, dataDir, s string, args ...interface{}) string {
	return fmt.Sprintf(`$env:KUBECONFIG=%s; %s`, ps.DoubleQuotePath(w.KubeconfigPath(h, dataDir)), w.K0sCmdf(`kubectl %s`, fmt.Sprintf(s, args...)))
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
	out, err := h.ExecOutput(ps.Cmd(`(Get-NetConnectionProfile -NetworkCategory Private | Select-Object -First 1).InterfaceAlias`))
	if err != nil || strings.TrimSpace(out) == "" {
		out, err = h.ExecOutput(ps.Cmd(`(Get-NetConnectionProfile | Select-Object -First 1).InterfaceAlias`))
	}
	if err != nil || strings.TrimSpace(out) == "" {
		return "", fmt.Errorf("failed to detect a private network interface, define the host privateInterface manually: %w", err)
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	if sc.Scan() {
		return strings.TrimSpace(sc.Text()), nil
	}
	return "", fmt.Errorf("failed to detect a private network interface")
}

// PrivateAddress resolves internal ip from private interface (not implemented for Windows yet)
func (w *BaseWindows) PrivateAddress(h os.Host, iface, publicip string) (string, error) {
	ip, err := h.ExecOutput(ps.Cmd(fmt.Sprintf(`(Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias %s).IPAddress`, ps.SingleQuote(iface))))
	if err != nil || strings.TrimSpace(ip) == "" {
		if !strings.HasPrefix(iface, "vEthernet") {
			ve := fmt.Sprintf("vEthernet (%s)", iface)
			ip, err = h.ExecOutput(ps.Cmd(fmt.Sprintf(`(Get-NetIPAddress -AddressFamily IPv4 -InterfaceAlias %s).IPAddress`, ps.SingleQuote(ve))))
		}
	}
	if err != nil {
		return "", fmt.Errorf("failed to get IP address for interface %s: %w", iface, err)
	}
	addr := strings.TrimSpace(ip)
	if addr != "" && addr != publicip {
		return addr, nil
	}
	return "", fmt.Errorf("not found")
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
'@ -Encoding ascii`, ps.DoubleQuotePath(tmpf), content))); err != nil {
		return err
	}

	// Atomically move if destination does not exist
	script := ps.Cmd(fmt.Sprintf(`if (!(Test-Path -Path %s)) { Move-Item -Path %s -Destination %s } else { Remove-Item -Path %s -Force }`, ps.DoubleQuotePath(path), ps.DoubleQuotePath(tmpf), ps.DoubleQuotePath(path), ps.DoubleQuotePath(tmpf)))
	if err := h.Exec(script); err != nil {
		return fmt.Errorf("upsert failed: %w", err)
	}

	// Ensure temp file is gone
	if h.Exec(ps.Cmd(fmt.Sprintf(`Test-Path -Path %s`, ps.DoubleQuotePath(tmpf)))) == nil {
		return fmt.Errorf("upsert failed")
	}
	return nil
}

func (w *BaseWindows) DeleteDir(h os.Host, path string, opts ...exec.Option) error {
	return h.Exec(ps.Cmd(fmt.Sprintf(`Remove-Item -Recurse -Force -Path %s`, ps.DoubleQuotePath(path))), opts...)
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
