package phase

import (
	"context"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	rigos "github.com/k0sproject/rig/v2/os"

	// anonymous import is needed to load the os configurers
	_ "github.com/k0sproject/k0sctl/configurer"
	// anonymous import is needed to load the os configurers
	_ "github.com/k0sproject/k0sctl/configurer/linux"
	// anonymous import is needed to load the os configurers
	_ "github.com/k0sproject/k0sctl/configurer/linux/enterpriselinux"
	// anonymous import is needed to load the os configurers
	_ "github.com/k0sproject/k0sctl/configurer/windows"

	log "github.com/sirupsen/logrus"
)

// DetectOS performs remote OS detection
type DetectOS struct {
	GenericPhase
}

// Title for the phase
func (p *DetectOS) Title() string {
	return "Detect host operating systems"
}

// Run the phase
func (p *DetectOS) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.Config.Spec.Hosts, func(_ context.Context, h *cluster.Host) error {
		if h.OSIDOverride != "" {
			log.Infof("%s: OS ID has been manually set to %s", h, h.OSIDOverride)
		}
		if err := h.ResolveConfigurer(); err != nil {
			// ID_LIKE fallback only applies to detected releases, not to a
			// manually forced OS ID.
			if h.OSIDOverride == "" {
				if release, osErr := h.OS(); osErr == nil && len(release.IDLike) > 0 {
					osStr := release.String()
					log.Debugf("%s: trying to find a fallback OS support module for %s using os-release ID_LIKE %v", h, osStr, release.IDLike)
					for _, id := range release.IDLike {
						h.OSRelease = &rigos.Release{ID: id, IDLike: release.IDLike, Name: release.Name, Version: release.Version}
						if err := h.ResolveConfigurer(); err == nil {
							log.Warnf("%s: using '%s' as OS support fallback for %s", h, id, osStr)
							return nil
						}
					}
					// No fallback matched; restore the detected release so the
					// host is not left with a synthetic candidate.
					h.OSRelease = release
				}
			}
			return err
		}
		log.Infof("%s: is running %s", h, h.OSRelease.String())

		// Needed to make configurer.K0sBinaryPath() to work inside the configurer itself as it can't call host.K0sInstallLocation().
		log.Debugf("%s: k0s install path is %s", h, h.K0sInstallLocation())
		h.Configurer.SetPath("K0sBinaryPath", h.K0sInstallLocation())

		return nil
	})
}

// After runs the per-host "connect: after" hooks once OS detection has succeeded.
func (p *DetectOS) After() error {
	return p.runHooks(context.Background(), "connect", "after", p.Config.Spec.Hosts...)
}
