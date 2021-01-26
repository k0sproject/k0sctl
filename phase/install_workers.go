package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// InstallWorkers installs k0s on worker hosts and joins them to the cluster
type InstallWorkers struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *InstallWorkers) Title() string {
	return "Install workers"
}

// Prepare the phase
func (p *InstallWorkers) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Workers()

	return nil
}

// ShouldRun is true when there are workers
func (p *InstallWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *InstallWorkers) Run() error {
	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		if h.Metadata.K0sRunningVersion == "" {
			log.Infof("%s: writing worker join token", h)
			if err := h.Configurer.WriteFile(h.K0sJoinTokenPath(), p.Config.Spec.K0s.Metadata.WorkerToken, "0640"); err != nil {
				return err
			}

			log.Infof("%s: installing k0s worker", h)
			if err := h.Exec(h.K0sInstallCommand(false)); err != nil {
				return err
			}
			log.Infof("%s: starting service", h)
			if err := h.Configurer.StartService("k0s"); err != nil {
				return err
			}
			h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
		} else {
			log.Infof("%s: k0s worker service already running", h)
		}

		return nil
	})
}
