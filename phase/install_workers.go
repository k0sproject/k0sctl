package phase

import (
	"context"
	"fmt"
	"strings"
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
	_ = p.After()
	_ = p.hosts.Filter(func(h *cluster.Host) bool {
		return !h.Metadata.Ready
	}).ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: cleaning up", h)
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %v", h, err)
			}
		}
		if h.Metadata.K0sInstalled && p.IsWet() {
			if err := h.Exec(h.Configurer.K0sCmdf("reset --data-dir=%s", h.K0sDataDir()), exec.Sudo(h)); err != nil {
				log.Warnf("%s: k0s reset failed", h)
			}
		}
		return nil
	})
}

func (p *InstallWorkers) After() error {
	if NoWait {
		for _, h := range p.hosts {
			if h.Metadata.K0sJoinToken != "" {
				log.Warnf("%s: --no-wait given, created join tokens will remain valid for 10 minutes", p.leader)
				break
			}
		}
		return nil
	}
	for i, h := range p.hosts {
		if h.Metadata.K0sJoinTokenID == "" {
			continue
		}
		h.Metadata.K0sJoinToken = ""
		err := p.Wet(p.leader, fmt.Sprintf("invalidate k0s join token for worker %s", h), func() error {
			log.Debugf("%s: invalidating join token for worker %d", p.leader, i+1)
			return p.leader.Exec(p.leader.Configurer.K0sCmdf("token invalidate --data-dir=%s %s", p.leader.K0sDataDir(), h.Metadata.K0sJoinTokenID), exec.Sudo(p.leader))
		})
		if err != nil {
			log.Warnf("%s: failed to invalidate worker join token: %v", p.leader, err)
		}
		_ = p.Wet(h, "overwrite k0s join token file", func() error {

			if err := h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), "# overwritten by k0sctl after join\n", "0600"); err != nil {
				log.Warnf("%s: failed to overwrite the join token file at %s", h, h.K0sJoinTokenPath())
			}
			return nil
		})
	}
	return nil
}

// Run the phase
func (p *InstallWorkers) Run() error {
	url := p.Config.Spec.InternalKubeAPIURL()
	healthz := fmt.Sprintf("%s/healthz", url)

	err := p.parallelDo(p.hosts, func(h *cluster.Host) error {
		if p.IsWet() || !p.leader.Metadata.DryRunFakeLeader {
			log.Infof("%s: validating api connection to %s", h, url)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := retry.Context(ctx, node.HTTPStatusFunc(h, healthz, 200, 401)); err != nil {
				return fmt.Errorf("failed to connect from worker to kubernetes api at %s - check networking", url)
			}
		} else {
			log.Warnf("%s: dry-run: skipping api connection validation to %s because cluster is not running", h, url)
		}
		return nil
	})

	if err != nil {
		return err
	}

	for i, h := range p.hosts {
		log.Infof("%s: generating a join token for worker %d", p.leader, i+1)
		err = p.Wet(p.leader, fmt.Sprintf("generate a k0s join token for worker %s", h), func() error {
			t, err := p.Config.Spec.K0s.GenerateToken(
				p.leader,
				"worker",
				time.Duration(10*time.Minute),
			)
			if err != nil {
				return err
			}
			h.Metadata.K0sJoinToken = t

			ti, err := cluster.TokenID(t)
			if err != nil {
				return err
			}
			h.Metadata.K0sJoinTokenID = ti

			log.Debugf("%s: join token ID: %s", h, ti)
			return nil
		}, func() error {
			h.Metadata.K0sJoinTokenID = "dry-run"
			return nil
		})
		if err != nil {
			return err
		}
	}

	return p.parallelDo(p.hosts, func(h *cluster.Host) error {
		err := p.Wet(h, fmt.Sprintf("write k0s join token to %s", h.K0sJoinTokenPath()), func() error {
			log.Infof("%s: writing join token", h)
			return h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), h.Metadata.K0sJoinToken, "0640")
		})
		if err != nil {
			return err
		}

		if sp, err := h.Configurer.ServiceScriptPath(h, h.K0sServiceName()); err == nil {
			if h.Configurer.ServiceIsRunning(h, h.K0sServiceName()) {
				err := p.Wet(h, "stop existing k0s service", func() error {
					log.Infof("%s: stopping service", h)
					return h.Configurer.StopService(h, h.K0sServiceName())
				})
				if err != nil {
					return err
				}
			}
			if h.Configurer.FileExist(h, sp) {
				err := p.Wet(h, "remove existing k0s service file", func() error {
					return h.Configurer.DeleteFile(h, sp)
				})
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
		err = p.Wet(h, fmt.Sprintf("install k0s worker with `%s`", strings.ReplaceAll(cmd, h.Configurer.K0sBinaryPath(), "k0s")), func() error {
			return h.Exec(cmd, exec.Sudo(h))
		})
		if err != nil {
			return err
		}

		h.Metadata.K0sInstalled = true

		if len(h.Environment) > 0 {
			err := p.Wet(h, "update service environment variables", func() error {
				log.Infof("%s: updating service environment", h)
				return h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment)
			})
			if err != nil {
				return err
			}
		}

		if p.IsWet() {
			log.Infof("%s: starting service", h)
			if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
				return err
			}
		}

		if NoWait {
			log.Debugf("%s: not waiting because --no-wait given", h)
			h.Metadata.Ready = true
		} else {
			log.Infof("%s: waiting for node to become ready", h)

			if p.IsWet() {
				if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.KubeNodeReadyFunc(h)); err != nil {
					return err
				}
				h.Metadata.Ready = true
			}
		}

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version

		return nil
	})
}
