package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// ConfigureK0s connects to each of the hosts
type ConfigureK0s struct {
	GenericPhase
	k0sconfig string
}

func (p *ConfigureK0s) Title() string {
	return "Configure K0s"
}

func (p *ConfigureK0s) Run() error {
	if len(p.Config.Spec.K0s.Config) == 0 {
		leader := p.Config.Spec.K0sLeader()
		log.Infof("%s: generating default configuration", leader)
		cfg, err := leader.ExecOutput("k0s default-config")
		if err != nil {
			return err
		}
		p.k0sconfig = cfg
	} else {
		b, err := yaml.Marshal(p.Config.Spec.K0s.Config)
		if err != nil {
			return err
		}
		p.k0sconfig = string(b)
	}

	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	return controllers.ParallelEach(p.configureK0s)
}

func (p *ConfigureK0s) configureK0s(h *cluster.Host) error {
	log.Infof("%s: writing k0s config", h)
	return h.Configurer.WriteFile(h.Configurer.K0sConfigPath(), p.k0sconfig, "0700")
}
