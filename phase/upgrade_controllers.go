package phase

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
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
func (p *UpgradeControllers) Prepare(config *config.Cluster) error {
	log.Debugf("UpgradeControllers phase prep starting")
	p.Config = config
	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	log.Debugf("%d controllers in total", len(controllers))
	p.hosts = controllers.Filter(func(h *cluster.Host) bool {
		return h.Metadata.NeedsUpgrade
	})
	log.Debugf("UpgradeControllers phase prepared, %d controllers needs upgrade", len(p.hosts))
	return nil
}

// ShouldRun is true when there are controllers that needs to be upgraded
func (p *UpgradeControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UpgradeControllers) Run() error {
	for _, h := range p.hosts {
		log.Infof("%s: starting upgrade", h)
		if p.needsMigration(h) {
			if err := p.migrateService(h); err != nil {
				return err
			}
		} else {
			if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
				return err
			}
		}
		if err := h.UpdateK0sBinary(p.Config.Spec.K0s.Version); err != nil {
			return err
		}
		if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
			return err
		}
		log.Infof("%s: waiting for the k0s service to start", h)
		if err := h.WaitK0sServiceRunning(); err != nil {
			return err
		}
		port := p.Config.Spec.K0s.Config.Dig("spec", "api", "port").(int)
		if port == 0 {
			port = 6443
		}
		if err := h.WaitKubeAPIReady(port); err != nil {
			return err
		}

	}
	return nil
}

func (p *UpgradeControllers) needsMigration(h *cluster.Host) bool {
	log.Debugf("%s: checking need for 0.10 --> 0.11 migration", h)
	c, _ := semver.NewConstraint("< 0.11-0")
	current, err := semver.NewVersion(h.Metadata.K0sRunningVersion)
	if err != nil {
		log.Warnf("%s: failed to parse version info: %s", h, err.Error())
		return false
	}

	return c.Check(current)
}

func (p *UpgradeControllers) migrateService(h *cluster.Host) error {

	log.Infof("%s: updating legacy 'k0sserver' service to '%s'", h, h.K0sServiceName())
	if err := h.Configurer.StopService(h, "k0sserver"); err != nil {
		return err
	}
	sp, err := h.Configurer.ServiceScriptPath(h, "k0sserver")
	if err != nil {
		return err
	}
	if sp == "" {
		return fmt.Errorf("service script path resolved to empty string")
	}
	log.Debugf("%s: found old service path: %s", h, sp)
	newPath := strings.Replace(sp, "k0sserver", h.K0sServiceName(), 1)
	if err != nil {
		return err
	}
	if err := h.Configurer.MoveFile(h, sp, newPath); err != nil {
		return err
	}

	return nil
}
