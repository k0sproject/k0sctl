package linux

import (
	"context"

	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// Debian provides OS support for Debian systems
type Debian struct {
	configurer.Linux
}

var _ configurer.Configurer = (*Debian)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "debian"
		},
		func() any {
			return &Debian{}
		},
	)
}

// InstallPackage installs packages via apt-get
func (l *Debian) InstallPackage(h configurer.Host, pkg ...string) error {
	pm := h.Sudo().PackageManager()
	ctx := context.Background()
	if err := pm.Update(ctx); err != nil {
		return err
	}
	return pm.Install(ctx, pkg...)
}
