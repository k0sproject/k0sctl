package phase

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// UpgradeWorkers upgrades workers in batches
type UpgradeWorkers struct {
	GenericPhase

	NoDrain bool

	hosts  cluster.Hosts
	leader *cluster.Host

	// keepCordoned holds the hosts that did not come back with k0s running
	// and the node ready after the upgrade. CleanUp leaves them cordoned so
	// that no workloads are scheduled on them while their node can still
	// look ready.
	keepCordoned sync.Map

	// failed holds the hosts on which a step of the upgrade failed. The
	// steps run one at a time for a batch of hosts and continue after
	// errors, so the remaining steps are skipped for these hosts.
	failed sync.Map
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
				log.Warnf("%s: failed to get service %s: %v", h, h.K0sServiceName(), err)
			} else if err := svc.SetEnvironment(context.Background(), map[string]string{}); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
		if _, ok := p.keepCordoned.Load(h); ok {
			log.Warnf("%s: leaving the node cordoned because it did not come back healthy after the upgrade", h)
			return nil
		}
		_ = p.leader.UncordonNode(h)
		// A worker that failed before uncordonWorker still has the eviction taint added by drainWorker.
		if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
			if err := p.leader.RemoveTaint(h, t.String()); err != nil {
				log.Warnf("%s: failed to remove taint %s: %s", h, t.String(), err.Error())
			}
		}
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
		p.step(p.start),
		p.step(p.cordonWorker),
		p.step(p.drainWorker),
		p.step(p.upgradeWorker),
		p.step(p.uncordonWorker),
		p.step(p.finish),
	)
}

// step wraps an upgrade step so that it is skipped for hosts on which an
// earlier step failed. Without it, a worker that failed to drain would still
// be upgraded, and one that failed to upgrade would be uncordoned and
// reported as upgraded.
func (p *UpgradeWorkers) step(fn func(context.Context, *cluster.Host) error) func(context.Context, *cluster.Host) error {
	return func(ctx context.Context, h *cluster.Host) error {
		if _, failed := p.failed.Load(h); failed {
			return nil
		}
		if err := fn(ctx, h); err != nil {
			p.failed.Store(h, struct{}{})
			return err
		}
		return nil
	}
}

func (p *UpgradeWorkers) cordonWorker(_ context.Context, h *cluster.Host) error {
	if p.NoDrain {
		log.Debugf("%s: not cordoning because --no-drain given", h)
		return nil
	}
	if !p.IsWet() {
		p.DryMsg(h, "cordon node")
		return nil
	}
	log.Debugf("%s: cordon", h)
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
	log.Debugf("%s: uncordon", h)
	if err := p.leader.UncordonNode(h); err != nil {
		return fmt.Errorf("uncordon node: %w", err)
	}
	if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
		log.Debugf("%s: remove taint: %s", h, t.String())
		if err := p.leader.RemoveTaint(h, t.String()); err != nil {
			return fmt.Errorf("remove taint: %w", err)
		}
	}
	return nil
}

func (p *UpgradeWorkers) drainWorker(_ context.Context, h *cluster.Host) error {
	if p.NoDrain {
		log.Debugf("%s: not draining because --no-drain given", h)
		return nil
	}
	if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
		log.Debugf("%s: add taint: %s", h, t.String())
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
	log.Debugf("%s: drain", h)
	if err := p.leader.DrainNode(h, p.Config.Spec.Options.Drain); err != nil {
		return fmt.Errorf("drain node: %w", err)
	}
	return nil
}

func (p *UpgradeWorkers) start(_ context.Context, h *cluster.Host) error {
	log.Infof("%s: starting upgrade", h)
	return nil
}

func (p *UpgradeWorkers) finish(_ context.Context, h *cluster.Host) error {
	log.Infof("%s: upgrade finished", h)
	return nil
}

func (p *UpgradeWorkers) upgradeWorker(ctx context.Context, h *cluster.Host) error {
	svc, svcErr := h.Sudo().Service(h.K0sServiceName())
	if svcErr != nil {
		return fmt.Errorf("get service %s: %w", h.K0sServiceName(), svcErr)
	}

	log.Debugf("%s: stop service", h)
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
		log.Debugf("%s: update binary", h)
		err = p.Wet(h, "replace k0s binary", func() error {
			return h.UpdateK0sBinary(h.Metadata.K0sBinaryTempFile, p.Config.Spec.K0s.Version)
		})
		if err != nil {
			return err
		}
		// Clear the temp file metadata after successful update to avoid stale paths.
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

	log.Debugf("%s: restart service", h)
	err = p.Wet(h, "restart k0s service", func() error {
		// The kubelet runs the version of the k0s binary on the host, which can differ from spec.k0s.version
		// with useExistingK0s or a custom k0sBinaryPath.
		binaryVersion, err := h.K0sBinaryVersion()
		if err != nil {
			return fmt.Errorf("get k0s binary version: %w", err)
		}
		if err := svc.Start(ctx); err != nil {
			return err
		}
		if NoWait {
			log.Debugf("%s: not waiting because --no-wait given", h)
		} else {
			log.Infof("%s: waiting for node to become ready again", h)
			if err := waitUpgradedWorker(ctx, h, binaryVersion); err != nil {
				p.keepCordoned.Store(h, struct{}{})
				return err
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

// maxK0sRestarts is how many times k0s may restart on an upgraded worker while
// waiting for the node to become ready. The wait fails on the next restart.
const maxK0sRestarts = 3

// waitUpgradedWorker waits until k0s is running on the restarted worker and its
// node is ready on the kubelet version v.
//
// A ready node does not mean that k0s is running: the service manager leaves
// the kubelet running when k0s exits and restarts k0s only after a delay. So
// k0s is checked again once the node is ready, and if it has restarted in the
// meantime, the wait starts over.
func waitUpgradedWorker(ctx context.Context, h *cluster.Host, v *version.Version) error {
	waitK0s := func() (int, error) {
		var pid int
		err := retry.WithDefaultTimeout(ctx, func(ctx context.Context) error {
			var err error
			pid, err = node.K0sPID(ctx, h)
			return err
		})
		return pid, err
	}

	pid, err := waitK0s()
	if err != nil {
		return fmt.Errorf("k0s did not start: %w", err)
	}

	restarts := 0
	for {
		if err := retry.WithDefaultTimeout(ctx, node.KubeNodeVersionReadyFunc(h, v)); err != nil {
			return fmt.Errorf("node did not become ready: %w", err)
		}
		current, err := waitK0s()
		if err != nil {
			return fmt.Errorf("k0s is not running after the node became ready: %w", err)
		}
		if current == pid {
			return nil
		}
		restarts++
		if restarts > maxK0sRestarts {
			return fmt.Errorf("k0s keeps restarting after the upgrade (restarted %d times)", restarts)
		}
		log.Warnf("%s: k0s restarted while waiting for the node to become ready (pid %d, previously %d)", h, current, pid)
		pid = current
	}
}
