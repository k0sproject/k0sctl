package windows

import (
	"fmt"
	"slices"
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
	ps "github.com/k0sproject/rig/v2/powershell"
)

// Windows provides OS support for Windows systems
type Windows struct {
	configurer.BaseWindows
}

var _ configurer.Configurer = (*Windows)(nil)
var _ configurer.HostValidator = (*Windows)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "windows" || slices.Contains(r.IDLike, "windows")
		},
		func() any {
			return &Windows{}
		},
	)
}

func (c *Windows) ValidateHost(h configurer.Host) error {
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

func detectContainersFeatureState(h configurer.Host) (string, error) {
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
