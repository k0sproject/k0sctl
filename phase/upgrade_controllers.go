package phase

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
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
func (p *UpgradeControllers) Run() error {
	for _, h := range p.hosts {
		if !h.Configurer.FileExist(h, h.Metadata.K0sBinaryTempFile) {
			return fmt.Errorf("k0s binary tempfile not found on host")
		}
		log.Infof("%s: starting upgrade", h)
		if p.needsMigration(h) {
			if err := p.migrateService(h); err != nil {
				return err
			}
		} else {
			log.Debugf("%s: stop service", h)
			if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
				return err
			}
		}
		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
			return fmt.Errorf("wait for k0s service stop: %w", err)
		}
		version, err := version.NewVersion(p.Config.Spec.K0s.Version)
		if err != nil {
			return err
		}
		log.Debugf("%s: update binary", h)
		if err := h.UpdateK0sBinary(h.Metadata.K0sBinaryTempFile, version); err != nil {
			return err
		}

		if len(h.Environment) > 0 {
			log.Infof("%s: updating service environment", h)
			if err := h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment); err != nil {
				return err
			}
		}

		log.Debugf("%s: restart service", h)
		if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
			return err
		}
		log.Infof("%s: waiting for the k0s service to start", h)
		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.ServiceRunningFunc(h, h.K0sServiceName())); err != nil {
			return fmt.Errorf("k0s service start: %w", err)
		}
		port := 6443
		if p, ok := p.Config.Spec.K0s.Config.Dig("spec", "api", "port").(int); ok {
			port = p
		}

		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.KubeAPIReadyFunc(h, port)); err != nil {
			return fmt.Errorf("kube api did not become ready: %w", err)
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
