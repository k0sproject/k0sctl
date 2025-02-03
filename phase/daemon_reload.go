package phase

import (
	"context"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
)

// DaemonReload phase runs `systemctl daemon-reload` or equivalent on all hosts.
type DaemonReload struct {
	GenericPhase
}

// Title for the phase
func (p *DaemonReload) Title() string {
	return "Reload service manager"
}

// ShouldRun is true when there are controllers that needs to be reset
func (p *DaemonReload) ShouldRun() bool {
	return len(p.Config.Spec.Hosts) > 0
}

// Run the phase
func (p *DaemonReload) Run(_ context.Context) error {
	return p.parallelDo(p.Config.Spec.Hosts, func(h *cluster.Host) error {
		log.Infof("%s: reloading service manager", h)
		if err := h.Configurer.DaemonReload(h); err != nil {
			log.Warnf("%s: failed to reload service manager: %s", h, err.Error())
		}
		return nil
	})
}
