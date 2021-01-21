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
		if !h.Metadata.K0sRunning {
			log.Infof("%s: installing k0s worker", h)
			if err := h.Exec(h.Configurer.K0sCmdf("install --role worker")); err != nil {
				return err
			}
		} else {
			log.Infof("%s: k0s service already running", h)
		}

		log.Infof("%s: updating join token", h)
		if err := h.Configurer.WriteFile(h.Configurer.K0sJoinTokenPath(), p.Config.Spec.K0s.Metadata.WorkerToken, "0640"); err != nil {
			return err
		}

		log.Infof("%s: updating service script", h)
		spath, err := h.Configurer.ServiceScriptPath("k0s")
		if err != nil {
			return err
		}
		if err := h.Configurer.ReplaceK0sTokenPath(spath); err != nil {
			return err
		}

		log.Infof("%s: reloading daemon configuration", h)
		if err := h.Configurer.DaemonReload(); err != nil {
			return err
		}

		if !h.Metadata.K0sRunning {
			log.Infof("%s: starting service", h)
			if err := h.Configurer.StartService("k0s"); err != nil {
				return err
			}
		} else {
			log.Infof("%s: restarting service", h)
			if err := h.Configurer.RestartService("k0s"); err != nil {
				return err
			}
		}

		h.Metadata.K0sRunning = true

		return nil
	})
}
