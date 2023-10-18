package phase

import (
	"context"
	"fmt"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
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
func (p *InstallWorkers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Workers().Filter(func(h *cluster.Host) bool {
		return !h.Reset && !h.Metadata.NeedsUpgrade && (h.Metadata.K0sRunningVersion == nil || !h.Metadata.Ready)
	})
	p.leader = p.Config.Spec.K0sLeader()

	return nil
}

// ShouldRun is true when there are workers
func (p *InstallWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// CleanUp attempts to clean up any changes after a failed install
func (p *InstallWorkers) CleanUp() {
	_ = p.hosts.Filter(func(h *cluster.Host) bool {
		return !h.Metadata.Ready
	}).ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: cleaning up", h)
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %v", h, err)
			}
		}
		if h.Metadata.K0sInstalled {
			if err := h.Exec(h.Configurer.K0sCmdf("reset --data-dir=%s", h.K0sDataDir()), exec.Sudo(h)); err != nil {
				log.Warnf("%s: k0s reset failed", h)
			}
		}
		return nil
	})
}

// Run the phase
func (p *InstallWorkers) Run() error {
	url := p.Config.Spec.KubeAPIURL()
	healthz := fmt.Sprintf("%s/healthz", url)

	err := p.parallelDo(p.hosts, func(h *cluster.Host) error {
		log.Infof("%s: validating api connection to %s", h, url)
		if err := retry.Times(context.Background(), 2, node.HTTPStatusFunc(h, healthz, 200, 401)); err != nil {
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
			if err := p.leader.Exec(p.leader.Configurer.K0sCmdf("token invalidate --data-dir=%s %s", p.leader.K0sDataDir(), tokenID), exec.Sudo(p.leader), exec.RedactString(token)); err != nil {
				log.Warnf("%s: failed to invalidate the worker join token", p.leader)
			}
		}()
	}

	return p.parallelDo(p.hosts, func(h *cluster.Host) error {
		log.Infof("%s: writing join token", h)
		if err := h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), token, "0640"); err != nil {
			return err
		}

		if !NoWait {
			defer func() {
				if err := h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), "# overwritten by k0sctl after join\n", "0600"); err != nil {
					log.Warnf("%s: failed to overwrite the join token file at %s", h, h.K0sJoinTokenPath())
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
		if Force {
			log.Warnf("%s: --force given, using k0s install with --force", h)
			h.InstallFlags.AddOrReplace("--force=true")
		}

		cmd, err := h.K0sInstallCommand()
		if err != nil {
			return err
		}
		if err = h.Exec(cmd); err != nil {
			return err
		}

		h.Metadata.K0sInstalled = true

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

			if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.KubeNodeReadyFunc(h)); err != nil {
				return err
			}
			h.Metadata.Ready = true
		}

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version

		return nil
	})
}
