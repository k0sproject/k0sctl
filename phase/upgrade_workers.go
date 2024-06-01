package phase

import (
	"context"
	"fmt"
	"math"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	log "github.com/sirupsen/logrus"
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
	var workers cluster.Hosts = p.Config.Spec.Hosts.Workers()
	log.Debugf("%d workers in total", len(workers))
	p.hosts = workers.Filter(func(h *cluster.Host) bool {
		if h.Metadata.K0sBinaryTempFile == "" {
			return false
		}
		return !h.Reset && h.Metadata.NeedsUpgrade
	})
	err := p.parallelDo(p.hosts, func(h *cluster.Host) error {
		if !h.Configurer.FileExist(h, h.Metadata.K0sBinaryTempFile) {
			return fmt.Errorf("k0s binary tempfile not found on host")
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

// CleanUp cleans up the environment override files on hosts
func (p *UpgradeWorkers) CleanUp() {
	if !p.IsWet() {
		return
	}
	_ = p.parallelDo(p.hosts, func(h *cluster.Host) error {
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
		_ = p.leader.UncordonNode(h)
		return nil
	})
}

// Run the phase
func (p *UpgradeWorkers) Run() error {
	// Upgrade worker hosts parallelly in 10% chunks
	concurrentUpgrades := int(math.Floor(float64(len(p.hosts)) * 0.10))
	if concurrentUpgrades == 0 {
		concurrentUpgrades = 1
	}

	log.Infof("Upgrading max %d workers in parallel", concurrentUpgrades)
	return p.hosts.BatchedParallelEach(concurrentUpgrades,
		p.start,
		p.cordonWorker,
		p.drainWorker,
		p.upgradeWorker,
		p.uncordonWorker,
		p.finish,
	)
}

func (p *UpgradeWorkers) cordonWorker(h *cluster.Host) error {
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

func (p *UpgradeWorkers) uncordonWorker(h *cluster.Host) error {
	if !p.IsWet() {
		p.DryMsg(h, "uncordon node")
		return nil
	}
	log.Debugf("%s: uncordon", h)
	if err := p.leader.UncordonNode(h); err != nil {
		return fmt.Errorf("uncordon node: %w", err)
	}
	return nil
}

func (p *UpgradeWorkers) drainWorker(h *cluster.Host) error {
	if p.NoDrain {
		log.Debugf("%s: not draining because --no-drain given", h)
		return nil
	}
	if !p.IsWet() {
		p.DryMsg(h, "drain node")
		return nil
	}
	log.Debugf("%s: drain", h)
	upgradeSettings := p.Config.Spec.UpgradeSettings
	if upgradeSettings == nil {
		upgradeSettings = &cluster.UpgradeSettings{}
	}

	gracePeriod := upgradeSettings.DrainGracePeriod
	if gracePeriod == "" {
		gracePeriod = "120"
	}
	timeout := upgradeSettings.DrainTimeout
	if timeout == "" {
		timeout = "5m"
	}

	if err := p.leader.DrainNode(h, gracePeriod, timeout); err != nil {
		return fmt.Errorf("drain node: %w", err)
	}
	return nil
}

func (p *UpgradeWorkers) start(h *cluster.Host) error {
	log.Infof("%s: starting upgrade", h)
	return nil
}

func (p *UpgradeWorkers) finish(h *cluster.Host) error {
	log.Infof("%s: upgrade finished", h)
	return nil
}

func (p *UpgradeWorkers) upgradeWorker(h *cluster.Host) error {
	log.Debugf("%s: stop service", h)
	err := p.Wet(h, "stop k0s service", func() error {
		if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
			return err
		}

		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
			return err
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

	log.Debugf("%s: restart service", h)
	err = p.Wet(h, "restart k0s service", func() error {
		if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
			return err
		}
		if NoWait {
			log.Debugf("%s: not waiting because --no-wait given", h)
		} else {
			log.Infof("%s: waiting for node to become ready again", h)
			if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.KubeNodeReadyFunc(h)); err != nil {
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
