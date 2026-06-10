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
func (p *DaemonReload) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.Config.Spec.Hosts, func(_ context.Context, h *cluster.Host) error {
		log.Infof("%s: reloading service manager", h)
		svc, err := h.Sudo().Service("k0scontroller")
		if err != nil {
			log.Warnf("%s: failed to get service k0scontroller: %s", h, err.Error())
			return nil
		}
		if err := svc.DaemonReload(ctx); err != nil {
			log.Warnf("%s: failed to reload service manager: %s", h, err.Error())
		}
		return nil
	})
}
