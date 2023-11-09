package phase

import (
	"context"
	"fmt"
	"math"

	"github.com/gammazero/workerpool"
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
	log.Debugf("UpgradeWorkers phase prepared, %d workers needs upgrade", len(p.hosts))

	return nil
}

// ShouldRun is true when there are workers that needs to be upgraded
func (p *UpgradeWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// CleanUp cleans up the environment override files on hosts
func (p *UpgradeWorkers) CleanUp() {
	for _, h := range p.hosts {
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
	}
}

// Run the phase
func (p *UpgradeWorkers) Run() error {
	// Upgrade worker hosts parallelly in 10% chunks
	concurrentUpgrades := int(math.Floor(float64(len(p.hosts)) * 0.10))
	if concurrentUpgrades == 0 {
		concurrentUpgrades = 1
	}
	log.Infof("Upgrading max %d workers in parallel", concurrentUpgrades)
	wp := workerpool.New(concurrentUpgrades)
	errors := make(map[string]error)
	for _, w := range p.hosts {
		h := w
		wp.Submit(func() {
			err := p.upgradeWorker(h)
			if err != nil {
				errors[h.String()] = err
				log.Errorf("%s: upgrade failed: %s", h, err.Error())
			}
		})
	}
	wp.StopWait()

	if len(errors) > 0 {
		return fmt.Errorf("upgrading %d workers failed", len(errors))
	}
	return nil
}

func (p *UpgradeWorkers) upgradeWorker(h *cluster.Host) error {
	if !h.Configurer.FileExist(h, h.Metadata.K0sBinaryTempFile) {
		return fmt.Errorf("k0s binary tempfile not found on host")
	}

	log.Infof("%s: starting upgrade", h)

	if !p.NoDrain {
		log.Debugf("%s: draining...", h)
		err := p.Wet(h, "drain node", func() error {
			return p.leader.DrainNode(h)
		})
		if err != nil {
			return err
		}
		log.Debugf("%s: draining complete", h)
	}

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

	if !p.NoDrain {
		log.Debugf("%s: marking node schedulable again", h)
		err := p.Wet(h, "uncordon node", func() error {
			return p.leader.UncordonNode(h)
		})
		if err != nil {
			return fmt.Errorf("uncordon node: %w", err)
		}
	}
	h.Metadata.Ready = true
	log.Infof("%s: upgrade successful", h)
	return nil
}
