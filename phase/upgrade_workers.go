package phase

import (
	"fmt"
	"math"

	"github.com/gammazero/workerpool"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
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
func (p *UpgradeWorkers) Prepare(config *config.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()
	var workers cluster.Hosts = p.Config.Spec.Hosts.Workers()
	log.Debugf("%d controllers in total", len(workers))
	p.hosts = workers.Filter(func(h *cluster.Host) bool {
		return h.Metadata.NeedsUpgrade
	})
	log.Debugf("UpgradeWorkers phase prepared, %d workers needs upgrade", len(p.hosts))

	return nil
}

// ShouldRun is true when there are workers that needs to be upgraded
func (p *UpgradeWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UpgradeWorkers) Run() error {
	// Upgrade worker hosts parallelly in 10% chunks
	concurrentUpgrades := int(math.Floor(float64(len(p.hosts)) * 0.10))
	if concurrentUpgrades == 0 {
		concurrentUpgrades = 1
	}
	log.Infof("Upgrading %d workers in parallel", concurrentUpgrades)
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
	log.Infof("%s: upgrade starting", h)

	if !p.NoDrain {
		log.Debugf("%s: draining...", h)
		if err := p.leader.DrainNode(h); err != nil {
			return err
		}
		log.Debugf("%s: draining complete", h)
	}

	log.Debugf("%s: Update and restart service", h)
	if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
		return err
	}
	if err := h.WaitK0sServiceStopped(); err != nil {
		return err
	}
	if err := h.UpdateK0sBinary(p.Config.Spec.K0s.Version); err != nil {
		return err
	}
	if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
		return err
	}
	if !p.NoDrain {
		log.Debugf("%s: marking node schedulable again", h)
		if err := p.leader.UncordonNode(h); err != nil {
			return err
		}
	}
	if NoWait {
		log.Debugf("%s: not waiting because --no-wait given", h)
	} else {
		log.Infof("%s: waiting for node to become ready again", h)
		if err := p.Config.Spec.K0sLeader().WaitKubeNodeReady(h); err != nil {
			return err
		}
		h.Metadata.Ready = true
	}
	log.Infof("%s: upgrade successful", h)
	return nil
}
