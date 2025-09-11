package windows

import (
	"fmt"

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

func (c *Windows) WriteFile(h os.Host, path, content, mode string) error {
	err := h.Exec(ps.Cmd(fmt.Sprintf(`$Input | Out-File -FilePath %s`, ps.DoubleQuotePath(path))), exec.Stdin(content), exec.RedactString(content))
	if err != nil {
		return fmt.Errorf("failed to write to file %s: %w", path, err)
	}

	return nil
}
