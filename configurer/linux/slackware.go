package linux

import (
	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
	"github.com/k0sproject/rig/v2/sh"
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
	return h.Sudo().Exec(sh.CommandBuilder("slackpkg update && slackpkg install --priority ADD").Args(pkg...).String())
}
