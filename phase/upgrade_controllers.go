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

// UpgradeControllers upgrades the controllers one-by-one
type UpgradeControllers struct {
	GenericPhase

	hosts cluster.Hosts
}

// Title for the phase
func (p *UpgradeControllers) Title() string {
	return "Upgrade controllers"
}

// Prepare the phase
func (p *UpgradeControllers) Prepare(config *v1beta1.Cluster) error {
	log.Debugf("UpgradeControllers phase prep starting")
	p.Config = config
	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	log.Debugf("%d controllers in total", len(controllers))
	p.hosts = controllers.Filter(func(h *cluster.Host) bool {
		if h.Metadata.K0sBinaryTempFile == "" {
			return false
		}
		return !h.Reset && h.Metadata.NeedsUpgrade
	})
	log.Debugf("UpgradeControllers phase prepared, %d controllers needs upgrade", len(p.hosts))
	return nil
}

// ShouldRun is true when there are controllers that needs to be upgraded
func (p *UpgradeControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// CleanUp cleans up the environment override files on hosts
func (p *UpgradeControllers) CleanUp() {
	for _, h := range p.hosts {
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
	}
}

// Run the phase
func (p *UpgradeControllers) Run(ctx context.Context) error {
	leader := p.Config.Spec.K0sLeader()
	for _, h := range p.hosts {
		if !h.Configurer.FileExist(h, h.Metadata.K0sBinaryTempFile) {
			return fmt.Errorf("k0s binary tempfile not found on host")
		}
		log.Infof("%s: starting upgrade", h)
		err := p.Wet(h, "shut down k0s", func() error {
			if h.Role != "controller" {
				if err := leader.DrainNode(h); err != nil {
					log.Warnf("%s: failed to drain node %s: %s", leader, h, err.Error())
				}
				log.Debugf("%s: deleting daemonset pods", h)
				if err := leader.KillDaemonSetPods(h, false); err != nil {
					if Force {
						log.Warnf("%s: failed to delete daemonset pods gracefully, will use force: %s", h, err.Error())
						if err := leader.KillDaemonSetPods(h, true); err != nil {
							log.Warnf("%s: failed to delete daemonset pods forcefully: %s", h, err.Error())
						}
					} else {
						log.Warnf("%s: failed to delete daemonset pods gracefully: %s", h, err.Error())
					}
				}
			}

			log.Debugf("%s: stop service", h)
			if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
				return err
			}
			if err := retry.AdaptiveTimeout(ctx, retry.DefaultTimeout, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
				return fmt.Errorf("wait for k0s service stop: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		log.Debugf("%s: update binary", h)
		err = p.Wet(h, "replace k0s binary", func() error {
			return h.UpdateK0sBinary(h.Metadata.K0sBinaryTempFile, p.Config.Spec.K0s.Version)
		})
		if err != nil {
			return err
		}

		if len(h.Environment) > 0 {
			log.Infof("%s: updating service environment", h)
			err := p.Wet(h, "update service environment", func() error {
				return h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment)
			})
			if err != nil {
				return err
			}
		}

		err = p.Wet(h, "reinstall k0s service", func() error {
			if p.Config.Spec.K0s.DynamicConfig {
				h.InstallFlags.AddOrReplace("--enable-dynamic-config")
			}

			h.InstallFlags.AddOrReplace("--force")

			cmd, err := h.K0sInstallCommand()
			if err != nil {
				return err
			}
			if err := h.Exec(cmd, exec.Sudo(h)); err != nil {
				return fmt.Errorf("failed to reinstall k0s: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		h.Metadata.K0sInstalled = true

		log.Debugf("%s: restart service", h)
		err = p.Wet(h, "start k0s service with the new binary", func() error {
			if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
				return err
			}
			log.Infof("%s: waiting for the k0s service to start", h)
			if err := retry.AdaptiveTimeout(ctx, retry.DefaultTimeout, node.ServiceRunningFunc(h, h.K0sServiceName())); err != nil {
				return fmt.Errorf("k0s service start: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		if p.IsWet() {
			err := retry.AdaptiveTimeout(ctx, 30*time.Second, func(_ context.Context) error {
				out, err := h.ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get --raw='/readyz?verbose=true'"), exec.Sudo(h))
				if err != nil {
					return fmt.Errorf("readiness endpoint reports %q: %w", out, err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("controller did not reach ready state: %w", err)
			}
		}

		_ = p.Wet(h, "uncordon node", func() error {
			if err := leader.UncordonNode(h); err != nil {
				log.Warnf("%s: failed to uncordon node %s: %s", leader, h, err.Error())
			}
			return nil
		})

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
	}

	return nil
}
