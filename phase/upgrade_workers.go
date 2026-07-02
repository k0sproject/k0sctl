package phase

import (
	"context"
	"fmt"
	"math"

	log "github.com/k0sproject/k0sctl/internal/log"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/v2/cmd"
)

// UpgradeWorkers upgrades workers in batches
type UpgradeWorkers struct {
	GenericPhase

	NoDrain bool

	hosts  cluster.Hosts
	leader *cluster.Host
}

// Title for the phase
func (p *UpgradeWorkers) Title() string {
	return "Upgrade workers"
}

// Prepare the phase
func (p *UpgradeWorkers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()
	workers := p.Config.Spec.Hosts.Workers()
	log.Debugf("%d workers in total", len(workers))
	p.hosts = workers.Filter(func(h *cluster.Host) bool {
		return !h.Reset && h.Metadata.NeedsUpgrade
	})
	err := p.parallelDo(context.Background(), p.hosts, func(_ context.Context, h *cluster.Host) error {
		if h.Metadata.K0sBinaryTempFile != "" && !h.FS().FileExist(h.Metadata.K0sBinaryTempFile) {
			return fmt.Errorf("k0s binary tempfile not found: %s", h.Metadata.K0sBinaryTempFile)
		}
		return nil
	})
	if err != nil {
		return err
	}
	log.Debugf("UpgradeWorkers phase prepared, %d workers needs upgrade", len(p.hosts))

	return nil
}

// ShouldRun is true when there are workers that needs to be upgraded
func (p *UpgradeWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Before runs "before upgrade" hooks for worker hosts that need upgrade
func (p *UpgradeWorkers) Before() error {
	if len(p.hosts) == 0 {
		return nil
	}
	return p.runHooks(context.Background(), "upgrade", "before", p.hosts...)
}

// After runs "after upgrade" hooks for worker hosts that were upgraded
func (p *UpgradeWorkers) After() error {
	if len(p.hosts) == 0 {
		return nil
	}
	return p.runHooks(context.Background(), "upgrade", "after", p.hosts...)
}

// CleanUp cleans up the environment override files on hosts
func (p *UpgradeWorkers) CleanUp() {
	if !p.IsWet() {
		return
	}
	_ = p.parallelDo(context.Background(), p.hosts, func(_ context.Context, h *cluster.Host) error {
		if len(h.Environment) > 0 {
			if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
				h.Log().Warnf("failed to get service %s: %v", h.K0sServiceName(), err)
			} else if err := svc.SetEnvironment(context.Background(), map[string]string{}); err != nil {
				h.Log().Warnf("failed to clean up service environment: %s", err.Error())
			}
		}
		_ = p.leader.UncordonNode(h)
		return nil
	})
}

// Run the phase
func (p *UpgradeWorkers) Run(ctx context.Context) error {
	// Upgrade worker hosts parallelly in 10% chunks
	concurrentUpgrades := int(math.Floor(float64(len(p.hosts)) * float64(p.Config.Spec.Options.Concurrency.WorkerDisruptionPercent/100)))
	if concurrentUpgrades == 0 {
		concurrentUpgrades = 1
	}
	concurrentUpgrades = min(concurrentUpgrades, p.Config.Spec.Options.Concurrency.Limit)

	log.Infof("Upgrading max %d workers in parallel", concurrentUpgrades)
	return p.hosts.BatchedParallelEach(ctx, concurrentUpgrades,
		p.start,
		p.cordonWorker,
		p.drainWorker,
		p.upgradeWorker,
		p.uncordonWorker,
		p.finish,
	)
}

func (p *UpgradeWorkers) cordonWorker(_ context.Context, h *cluster.Host) error {
	if p.NoDrain {
		h.Log().Debugf("not cordoning because --no-drain given")
		return nil
	}
	if !p.IsWet() {
		p.DryMsg(h, "cordon node")
		return nil
	}
	h.Log().Debugf("cordon")
	if err := p.leader.CordonNode(h); err != nil {
		return fmt.Errorf("cordon node: %w", err)
	}
	return nil
}

func (p *UpgradeWorkers) uncordonWorker(_ context.Context, h *cluster.Host) error {
	if !p.IsWet() {
		p.DryMsg(h, "uncordon node")
		if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
			p.DryMsgf(h, "remove taint %s", t.String())
		}
		return nil
	}
	h.Log().Debugf("uncordon")
	if err := p.leader.UncordonNode(h); err != nil {
		return fmt.Errorf("uncordon node: %w", err)
	}
	if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
		h.Log().Debugf("remove taint: %s", t.String())
		if err := p.leader.RemoveTaint(h, t.String()); err != nil {
			return fmt.Errorf("remove taint: %w", err)
		}
	}
	return nil
}

func (p *UpgradeWorkers) drainWorker(_ context.Context, h *cluster.Host) error {
	if p.NoDrain {
		h.Log().Debugf("not draining because --no-drain given")
		return nil
	}
	if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
		h.Log().Debugf("add taint: %s", t.String())
		err := p.Wet(h, "add taint "+t.String(), func() error {
			if err := p.leader.AddTaint(h, t.String()); err != nil {
				return fmt.Errorf("add taint: %w", err)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if !p.IsWet() {
		p.DryMsg(h, "drain node")
		return nil
	}
	h.Log().Debugf("drain")
	if err := p.leader.DrainNode(h, p.Config.Spec.Options.Drain); err != nil {
		return fmt.Errorf("drain node: %w", err)
	}
	return nil
}

func (p *UpgradeWorkers) start(_ context.Context, h *cluster.Host) error {
	h.Log().Infof("starting upgrade")
	return nil
}

func (p *UpgradeWorkers) finish(_ context.Context, h *cluster.Host) error {
	h.Log().Infof("upgrade finished")
	return nil
}

func (p *UpgradeWorkers) upgradeWorker(ctx context.Context, h *cluster.Host) error {
	svc, svcErr := h.Sudo().Service(h.K0sServiceName())
	if svcErr != nil {
		return fmt.Errorf("get service %s: %w", h.K0sServiceName(), svcErr)
	}

	h.Log().Debugf("stop service")
	err := p.Wet(h, "stop k0s service", func() error {
		if err := svc.Stop(ctx); err != nil {
			return err
		}

		if err := retry.WithDefaultTimeout(ctx, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if h.Metadata.K0sBinaryTempFile != "" {
		h.Log().Debugf("update binary")
		err = p.Wet(h, "replace k0s binary", func() error {
			return h.UpdateK0sBinary(h.Metadata.K0sBinaryTempFile, p.Config.Spec.K0s.Version)
		})
		if err != nil {
			return err
		}
		// Clear the temp file metadata after successful update to avoid stale paths.
		h.Metadata.K0sBinaryTempFile = ""
	} else {
		h.Log().Debugf("binary already in-place at %s, skipping binary replacement", h.K0sInstallLocation())
	}

	if len(h.Environment) > 0 {
		h.Log().Infof("updating service environment")
		err := p.Wet(h, "update service environment", func() error {
			return svc.SetEnvironment(ctx, h.Environment)
		})
		if err != nil {
			return err
		}
	}

	// Reinstall the service even when no binary was staged, to apply any flag or configuration changes.
	err = p.Wet(h, "reinstall k0s service", func() error {
		h.InstallFlags.AddOrReplace("--force")

		installCmd, err := h.K0sInstallCommand()
		if err != nil {
			return err
		}
		sudo := h.Sudo()
		if h.IsWindows() {
			if err := sudo.Exec(installCmd, cmd.AllowWinStderr()); err != nil {
				return fmt.Errorf("failed to reinstall k0s: %w", err)
			}
			return nil
		}
		if err := sudo.Exec(installCmd); err != nil {
			return fmt.Errorf("failed to reinstall k0s: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	h.Metadata.K0sInstalled = true

	h.Log().Debugf("restart service")
	err = p.Wet(h, "restart k0s service", func() error {
		if err := svc.Start(ctx); err != nil {
			return err
		}
		if NoWait {
			h.Log().Debugf("not waiting because --no-wait given")
		} else {
			h.Log().Infof("waiting for node to become ready again")
			if err := retry.WithDefaultTimeout(ctx, node.KubeNodeReadyFunc(h)); err != nil {
				return fmt.Errorf("node did not become ready: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	h.Metadata.Ready = true
	return nil
}
