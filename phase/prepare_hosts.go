package phase

import (
	"github.com/k0sproject/k0sctl/config"
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

// Prepare the phase
func (p *PrepareHosts) Prepare(config *config.Cluster) error {
	for _, h := range config.Spec.Hosts {
		if len(h.Environment) > 0 || !h.UploadBinary || h.Configurer.IsContainer(h) {
			p.hosts = append(p.hosts, h)
		}
	}
	return nil
}

// ShouldRun is true when there are hosts to be prepared
func (p *PrepareHosts) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *PrepareHosts) Run() error {
	return p.hosts.ParallelEach(p.prepareHost)
}

func (p *PrepareHosts) prepareHost(h *cluster.Host) error {
	if len(h.Environment) > 0 {
		log.Infof("%s: updating environment", h)
		if err := h.Configurer.UpdateEnvironment(h, h.Environment); err != nil {
			return err
		}
	}

	if h.Role == "worker" && !h.UploadBinary && !h.Configurer.CommandExist(h, "curl") {
		log.Infof("%s: installing packages", h)
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
