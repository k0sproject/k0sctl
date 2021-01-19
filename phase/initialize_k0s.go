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

// InitializeK0s connects to each of the hosts
type InitializeK0s struct {
	GenericPhase
	host *cluster.Host
}

func (p *InitializeK0s) Title() string {
	return "Initialize K0s Cluster"
}

func (p *InitializeK0s) Prepare(config *config.Cluster) error {
	p.Config = config
	p.host = p.Config.Spec.K0sLeader()
	return nil
}

func (p *InitializeK0s) ShouldRun() bool {
	return p.host != nil
}

func (p *InitializeK0s) Run() error {
	if p.host.Metadata.K0sRunning {
		log.Infof("%s: reloading configuration", p.host)
		if err := p.host.Configurer.RestartService("k0s"); err != nil {
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
	}

	log.Infof("%s: installing k0s controller", p.host)
	if err := p.host.Exec(p.host.Configurer.K0sCmdf("install --role server")); err != nil {
		return err
	}

	if err := p.host.Configurer.StartService("k0s"); err != nil {
		return err
	}
	if err := p.waitK0s(); err != nil {
		return err
	}
	p.host.Metadata.K0sRunning = true
	p.host.Metadata.K0sVersion = p.Config.Spec.K0s.Version

	token, err := p.generateToken("controller")
	if err != nil {
		return err
	}
	p.Config.Spec.K0s.Metadata.ControllerToken = token

	token, err = p.generateToken("worker")
	if err != nil {
		return err
	}
	p.Config.Spec.K0s.Metadata.WorkerToken = token

	return nil
}

func (p *InitializeK0s) waitK0s() error {
	return retry.Do(
		func() error {
			log.Infof("%s: waiting for k0s service to start", p.host)
			if !p.host.Configurer.ServiceIsRunning("k0s") {
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
	log.Infof("%s: generating worker join token", p.host)
	err = retry.Do(
		func() error {
			output, err := p.host.ExecOutput(p.host.Configurer.K0sCmdf("token create --role worker"), exec.HideOutput())
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
