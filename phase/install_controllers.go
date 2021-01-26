package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// InstallControllers installs k0s controllers and joins them to the cluster
type InstallControllers struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *InstallControllers) Title() string {
	return "Install controllers"
}

// Prepare the phase
func (p *InstallControllers) Prepare(config *config.Cluster) error {
	p.Config = config
	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	leader := p.Config.Spec.K0sLeader()
	p.hosts = controllers.Filter(func(h *cluster.Host) bool { return h != leader })

	return nil
}

// ShouldRun is true when there are controllers
func (p *InstallControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *InstallControllers) Run() error {
	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		if h.Metadata.K0sRunningVersion == "" {
			log.Infof("%s: writing join token", h)
			if err := h.Configurer.WriteFile(h.K0sJoinTokenPath(), p.Config.Spec.K0s.Metadata.ControllerToken, "0640"); err != nil {
				return err
			}

			log.Infof("%s: installing k0s controller", h)
			if err := h.Exec(h.K0sInstallCommand()); err != nil {
				return err
			}
			log.Infof("%s: starting service", h)
			if err := h.Configurer.StartService("k0s" + h.Role); err != nil {
				return err
			}
		} else {
			log.Infof("%s: k0s server already running", h)
		}

		return nil
	})
}
