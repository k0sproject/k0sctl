package phase

import (
	"context"
	"strings"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"

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
			if h.OSVersion.IDLike != "" {
				log.Debugf("%s: trying to find a fallback OS support module for %s using os-release ID_LIKE '%s'", h, h.OSVersion.String(), h.OSVersion.IDLike)
				for id := range strings.SplitSeq(h.OSVersion.IDLike, " ") {
					h.OSVersion.ID = id
					if err := h.ResolveConfigurer(); err == nil {
						log.Warnf("%s: using '%s' as OS support fallback for %s", h, id, h.OSVersion.String())
						return nil
					}
				}
			}
			return err
		}
		os := h.OSVersion.String()
		log.Infof("%s: is running %s", h, os)

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
