package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig/exec"
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
	if p.Config.Spec.K0s.Metadata.WorkerToken == "" {
		leader := p.Config.Spec.K0sLeader()
		log.Infof("%s: generating worker join token", leader)
		output, err := leader.ExecOutput(leader.Configurer.K0sCmdf("token create --role worker"), exec.HideOutput())
		if err != nil {
			return err
		}
		p.Config.Spec.K0s.Metadata.WorkerToken = output
	}

	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: installing k0s worker", h)
		if err := h.Exec(h.Configurer.K0sCmdf("install --role worker")); err != nil {
			return err
		}

		log.Infof("%s: updating join token", h)
		if err := h.Configurer.WriteFile(h.Configurer.K0sJoinTokenPath(), p.Config.Spec.K0s.Metadata.WorkerToken, "0640"); err != nil {
			return err
		}

		log.Infof("%s: updating service script", h)
		if err := h.Configurer.ReplaceK0sTokenPath(); err != nil {
			return err
		}

		log.Infof("%s: reloading daemon configuration", h)
		if err := h.Configurer.DaemonReload(); err != nil {
			return err
		}

		log.Infof("%s: starting service", h)
		if err := h.Configurer.StartService("k0s"); err != nil {
			return err
		}

		return nil
	})
}
