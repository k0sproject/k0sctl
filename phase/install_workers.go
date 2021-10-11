package phase

import (
	"fmt"
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

// CleanUp cleans up the environment override files on hosts
func (p *InstallWorkers) CleanUp() {
	for _, h := range p.hosts {
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
	}
}

// Run the phase
func (p *InstallWorkers) Run() error {
	url := p.Config.Spec.KubeAPIURL()
	healthz := fmt.Sprintf("%s/healthz", url)

	err := p.hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: validating api connection to %s", h, url)
		if err := h.WaitHTTPStatus(healthz, 200, 401); err != nil {
			return fmt.Errorf("failed to connect from worker to kubernetes api at %s - check networking", url)
		}
		return nil
	})

	if err != nil {
		return err
	}

	log.Infof("%s: generating token", p.leader)
	token, err := p.Config.Spec.K0s.GenerateToken(
		p.leader,
		"worker",
		time.Duration(10*len(p.hosts))*time.Minute,
	)
	if err != nil {
		return err
	}

	tokenID, err := cluster.TokenID(token)
	if err != nil {
		return err
	}
	log.Debugf("%s: join token ID: %s", p.leader, tokenID)

	if !NoWait {
		defer func() {
			if err := p.leader.Exec(p.leader.Configurer.K0sCmdf("token invalidate %s", tokenID), exec.Sudo(p.leader), exec.RedactString(token)); err != nil {
				log.Warnf("%s: failed to invalidate the worker join token", p.leader)
			}
		}()
	}

	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: writing join token", h)
		if err := h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), token, "0640"); err != nil {
			return err
		}

		if !NoWait {
			defer func() {
				if err := h.Configurer.DeleteFile(h, h.K0sJoinTokenPath()); err != nil {
					log.Warnf("%s: failed to clean up the join token file at %s", h, h.K0sJoinTokenPath())
				}
			}()
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

		if len(h.Environment) > 0 {
			log.Infof("%s: updating service environment", h)
			if err := h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment); err != nil {
				return err
			}
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
				return err
			}
			h.Metadata.Ready = true
		}

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version

		return nil
	})
}
