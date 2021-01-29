package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// PrepareHosts installs required packages and so on on the hosts.
type PrepareHosts struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *PrepareHosts) Title() string {
	return "Prepare hosts"
}

// ShouldRun is true when there are hosts to be prepared
func (p *PrepareHosts) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *PrepareHosts) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.prepareHost)
}

func (p *PrepareHosts) prepareHost(h *cluster.Host) error {
	if len(h.Environment) > 0 {
		log.Infof("%s: updating environment", h)
		if err := h.Configurer.UpdateEnvironment(h, h.Environment); err != nil {
			return err
		}
	}

	if h.IsController() || (h.Role == "worker" && !h.UploadBinary) {
		log.Infof("%s: installing %s", h, h.Configurer.WebRequestPackage())
		if err := h.Configurer.InstallPackage(h, h.Configurer.WebRequestPackage()); err != nil {
			return err
		}
	}

	if h.Configurer.IsContainer(h) {
		log.Infof("%s: is a container, applying fix", h)
		if err := h.Configurer.FixContainer(h); err != nil {
			return err
		}
	}

	return nil
}
