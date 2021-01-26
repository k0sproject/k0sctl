package phase

import (
	"fmt"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig/exec"
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
	p.host.Metadata.IsK0sLeader = true
	if p.host.Metadata.K0sRunningVersion != "" {
		log.Infof("%s: k0s already running, reloading configuration", p.host)
		if err := p.host.Configurer.RestartService(p.host.K0sServiceName()); err != nil {
			return err
		}
		if err := p.waitK0s(); err != nil {
			return err
		}
		token, err := p.generateToken("worker")
		if err != nil {
			return err
		}
		p.Config.Spec.K0s.Metadata.WorkerToken = token
		return nil
	}

	log.Infof("%s: installing k0s controller", p.host)
	if err := p.host.Exec(p.host.K0sInstallCommand()); err != nil {
		return err
	}

	if err := p.host.Configurer.StartService(p.host.K0sServiceName()); err != nil {
		return err
	}
	if err := p.waitK0s(); err != nil {
		return err
	}
	p.host.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
	p.host.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version

	if len(p.Config.Spec.Hosts.Controllers()) > 1 {
		token, err := p.generateToken("controller")
		if err != nil {
			return err
		}
		p.Config.Spec.K0s.Metadata.ControllerToken = token
	}

	if len(p.Config.Spec.Hosts.Workers()) > 0 {
		token, err := p.generateToken("worker")
		if err != nil {
			return err
		}
		p.Config.Spec.K0s.Metadata.WorkerToken = token
	}

	return nil
}

func (p *InitializeK0s) waitK0s() error {
	return retry.Do(
		func() error {
			log.Infof("%s: waiting for k0s service to start", p.host)
			if !p.host.Configurer.ServiceIsRunning(p.host.K0sServiceName()) {
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

func (p *InitializeK0s) generateToken(role string) (token string, err error) {
	log.Infof("%s: generating %s join token", p.host, role)
	err = retry.Do(
		func() error {
			output, err := p.host.ExecOutput(p.host.Configurer.K0sCmdf("token create --role %s", role), exec.HideOutput())
			if err != nil {
				return err
			}
			token = output
			return nil
		},
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(60),
	)
	return
}
