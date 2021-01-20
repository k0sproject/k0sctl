package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// PrepareHosts connects to each of the hosts
type PrepareHosts struct {
	GenericPhase
	hosts cluster.Hosts
}

func (p *PrepareHosts) Title() string {
	return "Prepare hosts"
}

func (p *PrepareHosts) Prepare(config *config.Cluster) error {
	for _, h := range config.Spec.Hosts {
		if len(h.Environment) > 0 || !h.UploadBinary {
			p.hosts = append(p.hosts, h)
		}
	}
	return nil
}

func (p *PrepareHosts) ShouldRun() bool {
	return len(p.hosts) > 0
}

func (p *PrepareHosts) Run() error {
	return p.hosts.ParallelEach(p.prepareHost)
}

func (p *PrepareHosts) prepareHost(h *cluster.Host) error {
	if len(h.Environment) > 0 {
		log.Infof("%s: updating environment", h)
		if err := h.Configurer.UpdateEnvironment(h.Environment); err != nil {
			return err
		}
	}

	if !h.UploadBinary {
		log.Infof("%s: installing packages", h)
		if err := h.Configurer.InstallPackage(h.Configurer.WebRequestPackage()); err != nil {
			return err
		}
	}

	return nil
}
