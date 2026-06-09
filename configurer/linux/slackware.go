package linux

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// Slackware provides OS support for Slackware Linux
type Slackware struct {
	BaseLinux
}

var _ configurer.Configurer = (*Slackware)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "slackware"
		},
		func() any {
			return &Slackware{}
		},
	)
}

// InstallPackage installs packages via slackpkg
func (l *Slackware) InstallPackage(h configurer.Host, pkg ...string) error {
	return h.Sudo().Exec(fmt.Sprintf("slackpkg update && slackpkg install --priority ADD %s", strings.Join(pkg, " ")))
}
