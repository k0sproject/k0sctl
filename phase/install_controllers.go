package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// InstallControllers connects to each of the hosts
type InstallControllers struct {
	GenericPhase
	hosts cluster.Hosts
}

func (p *InstallControllers) Title() string {
	return "Install controllers"
}

func (p *InstallControllers) Prepare(config *config.Cluster) error {
	p.Config = config
	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	leader := p.Config.Spec.K0sLeader()
	p.hosts = controllers.Filter(func(h *cluster.Host) bool { return h != leader })

	return nil
}

func (p *InstallControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

func (p *InstallControllers) Run() error {
	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: installing k0s controller", h)
		if err := h.Exec(h.Configurer.K0sCmdf("install --role server")); err != nil {
			return err
		}

		if err := h.Configurer.StartService("k0s"); err != nil {
			return err
		}

		log.Warnf("put the token somewhere")

		return nil
	})
}
