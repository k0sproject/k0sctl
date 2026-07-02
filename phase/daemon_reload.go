package phase

import (
	"context"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2/initsystem"
)

// DaemonReload phase runs `systemctl daemon-reload` or equivalent on hosts whose
// init system supports it (e.g. systemd). Hosts running OpenRC, WinSCM, or other
// init systems that do not implement ServiceManagerReloader are skipped.
type DaemonReload struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *DaemonReload) Title() string {
	return "Reload service manager"
}

// Prepare filters the host list to those whose init system implements ServiceManagerReloader.
func (p *DaemonReload) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	for _, h := range p.Config.Spec.Hosts {
		sudo := h.Sudo()
		mgr, err := sudo.ServiceManager()
		if err != nil {
			continue
		}
		if _, ok := mgr.(initsystem.ServiceManagerReloader); ok {
			p.hosts = append(p.hosts, h)
		}
	}
	return nil
}

// ShouldRun is true when at least one host supports daemon-reload.
func (p *DaemonReload) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *DaemonReload) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.hosts, func(ctx context.Context, h *cluster.Host) error {
		h.Log().Infof("reloading service manager")
		sudo := h.Sudo()
		mgr, err := sudo.ServiceManager()
		if err != nil {
			return nil
		}
		reloader, ok := mgr.(initsystem.ServiceManagerReloader)
		if !ok {
			return nil
		}
		if err := reloader.DaemonReload(ctx, sudo); err != nil {
			h.Log().Warnf("failed to reload service manager: %s", err.Error())
		}
		return nil
	})
}
