package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// GatherFacts connects to each of the hosts
type GatherFacts struct {
	GenericPhase
}

func (p *GatherFacts) Title() string {
	return "Gather host facts"
}

func (p *GatherFacts) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.investigateHost)
}

func (p *GatherFacts) investigateHost(h *cluster.Host) error {
	log.Infof("%s: investigating host", h)

	if h.Configurer.ServiceIsRunning("k0s") {
		h.Metadata.K0sRunning = true
		log.Infof("%s: k0s service is running", h)
	}

	if output, err := h.ExecOutput("k0s version"); err == nil {
		h.Metadata.K0sVersion = output
		log.Infof("%s: has k0s binary version %s", h, h.Metadata.K0sVersion)
	}

	return nil
}
