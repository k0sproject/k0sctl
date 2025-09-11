package windows

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	"github.com/k0sproject/rig/os/registry"
	ps "github.com/k0sproject/rig/pkg/powershell"
)

// Windows provides OS support for Windows systems
type Windows struct {
	os.Windows
	configurer.BaseWindows
}

var _ configurer.Configurer = (*Windows)(nil)
var _ configurer.HostValidator = (*Windows)(nil)

func init() {
	registry.RegisterOSModule(
		func(osv rig.OSVersion) bool {
			return osv.ID == "windows" || osv.IDLike == "windows"
		},
		func() interface{} {
			return &Windows{}
		},
	)
}

func (c *Windows) ValidateHost(h os.Host) error {
	state, err := detectContainersFeatureState(h)
	if err != nil {
		return err
	}

	switch strings.ToLower(state) {
	case "installed", "enabled":
		return nil
	}

	return fmt.Errorf(`windows feature "Containers" must be enabled (current state: %s)`, state)
}

func detectContainersFeatureState(h os.Host) (string, error) {
	commands := []string{
		`(Get-WindowsFeature -Name Containers -ErrorAction SilentlyContinue).InstallState`,
		`(Get-WindowsOptionalFeature -Online -FeatureName Containers -ErrorAction SilentlyContinue).State`,
	}

	var lastErr error
	for _, script := range commands {
		cmd := ps.Cmd(script)
		out, err := h.ExecOutput(cmd)
		if err != nil {
			lastErr = err
			continue
		}
		state := strings.TrimSpace(out)
		if state != "" {
			return state, nil
		}
		lastErr = fmt.Errorf("%s returned an empty state", script)
	}

	if lastErr != nil {
		return "", fmt.Errorf("failed to detect Containers feature state: %w", lastErr)
	}

	return "", fmt.Errorf("failed to detect Containers feature state")
}

func writeFileScript(path string) string {
	return fmt.Sprintf(`[System.IO.File]::WriteAllText(%s, [Console]::In.ReadToEnd(), [System.Text.UTF8Encoding]::new($false))`, ps.DoubleQuotePath(ps.ToWindowsPath(path)))
}

func (c *Windows) WriteFile(h os.Host, path, content, mode string) error {
	cmd := ps.Cmd(writeFileScript(path))
	err := h.Exec(cmd, exec.Stdin(content), exec.RedactString(content))
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", path, err)
	}

	return nil
}

// ServiceScriptPath synthesizes an identifier for the Windows service configuration.
// Windows services do not have init scripts, so we verify that the service exists
// and return a pseudo path that can be used for logging and detection.
func (c *Windows) ServiceScriptPath(h os.Host, service string) (string, error) {
	cmd := ps.Cmd(fmt.Sprintf(`sc.exe query %s | Out-Null`, ps.SingleQuote(service)))
	if err := h.Exec(cmd); err != nil {
		return "", fmt.Errorf("failed to find service %s: %w", service, err)
	}

	return fmt.Sprintf("winservice:%s", service), nil
}
