package phase

import (
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
		return nil
	}

	log.Infof("%s: installing k0s controller", p.host)
	if err := p.host.Exec(p.host.Configurer.K0sCmdf("install --role server")); err != nil {
		return err
	}

	if err := p.host.Configurer.StartService("k0s"); err != nil {
		return err
	}
	p.host.Metadata.K0sRunning = true
	p.host.Metadata.K0sVersion = p.Config.Spec.K0s.Version

	log.Infof("%s: generating controller join token", p.host)
	output, err := p.host.ExecOutput(p.host.Configurer.K0sCmdf("token create --role controller"), exec.HideOutput())
	if err != nil {
		return err
	}
	p.Config.Spec.K0s.Metadata.ControllerToken = output

	log.Infof("%s: generating worker join token", p.host)
	output, err = p.host.ExecOutput(p.host.Configurer.K0sCmdf("token create --role worker"), exec.HideOutput())
	if err != nil {
		return err
	}
	p.Config.Spec.K0s.Metadata.WorkerToken = output

	return nil
}
