package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// InstallWorkers connects to each of the hosts
type InstallWorkers struct {
	GenericPhase
	hosts cluster.Hosts
}

func (p *InstallWorkers) Title() string {
	return "Install workers"
}

func (p *InstallWorkers) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Workers()

	return nil
}

func (p *InstallWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

func (p *InstallWorkers) Run() error {
	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: installing k0s worker", h)
		if err := h.Exec(h.Configurer.K0sCmdf("install --role worker")); err != nil {
			return err
		}

		if err := h.Configurer.StartService("k0s"); err != nil {
			return err
		}

		log.Warnf("put the token somewhere")

		return nil
	})
}
