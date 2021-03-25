package phase

import (
	"time"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// InstallWorkers installs k0s on worker hosts and joins them to the cluster
type InstallWorkers struct {
	GenericPhase
	hosts  cluster.Hosts
	leader *cluster.Host
}

// Title for the phase
func (p *InstallWorkers) Title() string {
	return "Install workers"
}

// Prepare the phase
func (p *InstallWorkers) Prepare(config *config.Cluster) error {
	p.Config = config
	var workers cluster.Hosts = p.Config.Spec.Hosts.Workers()
	p.hosts = workers.Filter(func(h *cluster.Host) bool {
		return h.Metadata.K0sRunningVersion == "" || !h.Metadata.Ready
	})
	p.leader = p.Config.Spec.K0sLeader()

	return nil
}

// ShouldRun is true when there are workers
func (p *InstallWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *InstallWorkers) Run() error {
	log.Infof("%s: generating token", p.leader)
	token, err := p.Config.Spec.K0s.GenerateToken(
		p.leader,
		"worker",
		time.Duration(10*len(p.hosts))*time.Minute,
	)
	if err != nil {
		return err
	}

	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: writing join token", h)
		if err := h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), token, "0640"); err != nil {
			return err
		}

		if sp, err := h.Configurer.ServiceScriptPath(h, h.K0sServiceName()); err == nil {
			if h.Configurer.ServiceIsRunning(h, h.K0sServiceName()) {
				log.Infof("%s: stopping service", h)
				if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
					return err
				}
			}
			if h.Configurer.FileExist(h, sp) {
				err := h.Configurer.DeleteFile(h, sp)
				if err != nil {
					return err
				}
			}
		}

		log.Infof("%s: installing k0s worker", h)
		if err := h.Exec(h.K0sInstallCommand()); err != nil {
			return err
		}

		log.Infof("%s: starting service", h)
		if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
			return err
		}

		if NoWait {
			log.Debugf("%s: not waiting because --no-wait given", h)
		} else {
			log.Infof("%s: waiting for node to become ready", h)
			if err := p.Config.Spec.K0sLeader().WaitKubeNodeReady(h); err != nil {
				log.Errorf("%s: node not ready within timelimit", h)
				status, _ := h.Configurer.ServiceStatus(h, h.K0sServiceName())
				log.Errorf("%s: service status: %s", h, status)
				op, _ := p.Config.Spec.K0sLeader().ExecOutput(h.Configurer.KubectlCmdf("describe node,pod -A"), exec.HideOutput())
				log.Errorf("%s: node and pod status: %s", h, op)

				// FIXME: Needs to be handle in portable way, just for CI debugging purposes for now
				op, _ = h.ExecOutput("sudo journalctl -u k0sworker --no-pager --full", exec.HideOutput())
				log.Errorf("%s: full worker service journal logs:\n %s", h, op)

				return err
			}
			h.Metadata.Ready = true
		}

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version

		return nil
	})
}
