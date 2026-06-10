package linux

import (
	"context"

	"github.com/k0sproject/k0sctl/configurer"
	rigos "github.com/k0sproject/rig/v2/os"
)

// SLES provides OS support for SUSE Linux Enterprise Server
type SLES struct {
	BaseLinux
}

var _ configurer.Configurer = (*SLES)(nil)

func init() {
	configurer.RegisterOSModule(
		func(r *rigos.Release) bool {
			return r.ID == "sles"
		},
		func() any {
			return &SLES{}
		},
	)
}

// InstallPackage installs packages via zypper
func (l *SLES) InstallPackage(h configurer.Host, pkg ...string) error {
	pm := h.Sudo().PackageManager()
	ctx := context.Background()
	if err := pm.Update(ctx); err != nil {
		return err
	}
	return pm.Install(ctx, pkg...)
}
