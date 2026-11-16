package phase

import (
	"context"
	"fmt"
	"slices"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// UpgradeControllers upgrades the controllers one-by-one
type UpgradeControllers struct {
	GenericPhase

	// SkipVersionSkewCheck allows upgrading controllers by more than one minor version
	SkipVersionSkewCheck bool

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
	controllers := p.Config.Spec.Hosts.Controllers()
	log.Debugf("%d controllers in total", len(controllers))
	p.hosts = controllers.Filter(func(h *cluster.Host) bool {
		return !h.Reset && h.Metadata.NeedsUpgrade
	})
	log.Debugf("UpgradeControllers phase prepared, %d controllers needs upgrade", len(p.hosts))
	return nil
}

// ShouldRun is true when there are controllers that needs to be upgraded
func (p *UpgradeControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Before runs "before upgrade" hooks for controller hosts that need upgrade
func (p *UpgradeControllers) Before() error {
	if len(p.hosts) == 0 {
		return nil
	}
	return p.runHooks(context.Background(), "upgrade", "before", p.hosts...)
}

// After runs "after upgrade" hooks for controller hosts that were upgraded
func (p *UpgradeControllers) After() error {
	if len(p.hosts) == 0 {
		return nil
	}
	return p.runHooks(context.Background(), "upgrade", "after", p.hosts...)
}

// CleanUp cleans up the environment override files on hosts
func (p *UpgradeControllers) CleanUp() {
	for _, h := range p.hosts {
		if len(h.Environment) > 0 {
			if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to get service %s: %v", h, h.K0sServiceName(), err)
			} else if err := svc.SetEnvironment(context.Background(), map[string]string{}); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
	}
}

// validateStagedBinaries checks the version skew of the controllers that are upgraded with a binary from a custom
// k0sBinaryPath. The version of such a binary is not known before it is staged and can differ from the configured
// version, which is what the version skew check in the "Validate facts" phase had to assume.
func (p *UpgradeControllers) validateStagedBinaries() error {
	if p.SkipVersionSkewCheck {
		return nil
	}
	for _, h := range p.hosts {
		running := h.Metadata.K0sRunningVersion
		if h.K0sBinaryPath == "" || h.Metadata.K0sBinaryTempFile == "" || running == nil {
			continue
		}
		staged, err := stagedK0sBinaryVersion(h)
		if err != nil {
			return fmt.Errorf("%s: get the version of the staged k0s binary: %w", h, err)
		}
		if skew, ok := minorVersionSkew(running, staged); ok && skew > 1 {
			return controllerVersionSkewError(h, running, staged)
		}
	}
	return nil
}

// stagedK0sBinaryVersion returns the version the staged k0s binary of the host reports.
func stagedK0sBinaryVersion(h *cluster.Host) (*version.Version, error) {
	installed := h.K0sInstallLocation()
	h.Configurer.SetPath("K0sBinaryPath", h.Metadata.K0sBinaryTempFile)
	defer h.Configurer.SetPath("K0sBinaryPath", installed)
	return h.K0sBinaryVersion()
}

// Run the phase
func (p *UpgradeControllers) Run(ctx context.Context) error {
	// Before any controller is touched, so that a failure does not leave the control plane partially upgraded
	if err := p.validateStagedBinaries(); err != nil {
		return err
	}

	for _, h := range p.hosts {
		log.Infof("%s: starting upgrade", h)

		if h.Metadata.K0sBinaryTempFile != "" && !h.FS().FileExist(h.Metadata.K0sBinaryTempFile) {
			return fmt.Errorf("%s: k0s binary tempfile not found: %s", h, h.Metadata.K0sBinaryTempFile)
		}

		if t := p.Config.Spec.Options.EvictTaint; t.Enabled && t.ControllerWorkers && h.Role != "controller" {
			leader := p.Config.Spec.K0sLeader()
			err := p.Wet(leader, "apply taint to node", func() error {
				log.Warnf("%s: add taint %s on %s", leader, t.String(), h)
				if err := leader.AddTaint(h, t.String()); err != nil {
					return fmt.Errorf("add taint: %w", err)
				}
				log.Debugf("%s: wait for taint to be applied", h)
				err := retry.WithDefaultTimeout(ctx, func(_ context.Context) error {
					taints, err := leader.Taints(h)
					if err != nil {
						return fmt.Errorf("get taints: %w", err)
					}
					if !slices.Contains(taints, t.String()) {
						return fmt.Errorf("taint %s not found", t.String())
					}
					return nil
				})
				return err
			})
			if err != nil {
				return fmt.Errorf("apply taint: %w", err)
			}
		}

		log.Debugf("%s: stop service", h)
		svc, svcErr := h.Sudo().Service(h.K0sServiceName())
		if svcErr != nil {
			return fmt.Errorf("get service %s: %w", h.K0sServiceName(), svcErr)
		}
		err := p.Wet(h, "stop k0s service", func() error {
			if err := svc.Stop(ctx); err != nil {
				return err
			}
			if err := retry.WithDefaultTimeout(ctx, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
				return fmt.Errorf("wait for k0s service stop: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		if h.Metadata.K0sBinaryTempFile != "" {
			log.Debugf("%s: update binary", h)
			err = p.Wet(h, "replace k0s binary", func() error {
				return h.UpdateK0sBinary(h.Metadata.K0sBinaryTempFile, p.Config.Spec.K0s.Version)
			})
			if err != nil {
				return err
			}
			h.Metadata.K0sBinaryTempFile = ""
		} else {
			log.Debugf("%s: binary already in-place at %s, skipping binary replacement", h, h.K0sInstallLocation())
		}

		if len(h.Environment) > 0 {
			log.Infof("%s: updating service environment", h)
			err := p.Wet(h, "update service environment", func() error {
				return svc.SetEnvironment(ctx, h.Environment)
			})
			if err != nil {
				return err
			}
		}

		// Reinstall the service even when no binary was staged, to apply any flag or configuration changes.
		err = p.Wet(h, "reinstall k0s service", func() error {
			if p.Config.Spec.K0s.DynamicConfig {
				h.InstallFlags.AddOrReplace("--enable-dynamic-config")
			}

			h.InstallFlags.AddOrReplace("--force")

			cmd, err := h.K0sInstallCommand()
			if err != nil {
				return err
			}
			if err := h.Sudo().Exec(cmd); err != nil {
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
			if err := svc.Start(ctx); err != nil {
				return err
			}
			log.Infof("%s: waiting for the k0s service to start", h)
			if err := retry.WithDefaultTimeout(ctx, node.ServiceRunningFunc(h, h.K0sServiceName())); err != nil {
				return fmt.Errorf("k0s service start: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		if p.IsWet() {
			err := retry.WithDefaultTimeout(ctx, func(_ context.Context) error {
				out, err := h.Sudo().ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get --raw='/readyz?verbose=true'"))
				if err != nil {
					return fmt.Errorf("readiness endpoint reports %q: %w", out, err)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("controller did not reach ready state: %w", err)
			}
		}

		if t := p.Config.Spec.Options.EvictTaint; t.Enabled && t.ControllerWorkers && h.Role != "controller" {
			leader := p.Config.Spec.K0sLeader()
			err := p.Wet(leader, "remove taint from node", func() error {
				log.Infof("%s: remove taint %s on %s", leader, t.String(), h)
				if err := leader.RemoveTaint(h, t.String()); err != nil {
					return fmt.Errorf("remove taint: %w", err)
				}
				return nil
			})
			if err != nil {
				log.Warnf("%s: failed to remove taint %s on %s: %s", leader, t.String(), h, err.Error())
			}
		}

		h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
	}

	return nil
}
