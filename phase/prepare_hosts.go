package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// PrepareHosts installs required packages and so on on the hosts.
type PrepareHosts struct {
	GenericPhase
	hosts []*cluster.Host
}

// Title for the phase
func (p *PrepareHosts) Title() string {
	return "Prepare hosts"
}

// Prepare the phase
func (p *PrepareHosts) Prepare(config *config.Cluster) error {
	for _, h := range config.Spec.Hosts {
		if len(h.Environment) > 0 {
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
	return p.Config.Spec.Hosts.ParallelEach(p.prepareHost)
}

func (p *PrepareHosts) prepareHost(h *cluster.Host) error {
	log.Infof("%s: updating environment", h)

	return h.Configurer.UpdateEnvironment(h.Environment)
}
