package phase

import (
	"fmt"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// InitializeK0s sets up the "initial" k0s controller
type InitializeK0s struct {
	GenericPhase
	host *cluster.Host
}

// Title for the phase
func (p *InitializeK0s) Title() string {
	return "Initialize K0s Cluster"
}

// Prepare the phase
func (p *InitializeK0s) Prepare(config *config.Cluster) error {
	p.Config = config
	p.host = p.Config.Spec.K0sLeader()
	return nil
}

// ShouldRun is true when there is a leader host
func (p *InitializeK0s) ShouldRun() bool {
	return p.host != nil
}

// Run the phase
func (p *InitializeK0s) Run() error {
	h := p.host
	h.Metadata.IsK0sLeader = true
	if h.Metadata.K0sRunningVersion != "" {
		log.Infof("%s: k0s already running, reloading configuration", h)
		if err := h.Configurer.RestartService(h, h.K0sServiceName()); err != nil {
			return err
		}
		if err := p.waitK0s(); err != nil {
			return err
		}
		return nil
	}

	log.Infof("%s: installing k0s controller", h)
	if err := h.Exec(h.K0sInstallCommand()); err != nil {
		return err
	}

	if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
		return err
	}
	if err := p.waitK0s(); err != nil {
		return err
	}
	h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
	h.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version

	log.Infof("%s: installing kubectl", h)
	return h.Configurer.InstallKubectl(h)
}

func (p *InitializeK0s) waitK0s() error {
	return retry.Do(
		func() error {
			log.Infof("%s: waiting for k0s service to start", p.host)
			if !p.host.Configurer.ServiceIsRunning(p.host, p.host.K0sServiceName()) {
				return fmt.Errorf("not running")
			}
			return nil
		},
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(60),
	)
}
