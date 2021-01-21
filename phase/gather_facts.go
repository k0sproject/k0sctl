package phase

import (
	"strings"

	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// GatherFacts gathers information about hosts, such as if k0s is already up and running
type GatherFacts struct {
	GenericPhase
}

// Title for the phase
func (p *GatherFacts) Title() string {
	return "Gather host facts"
}

// Run the phase
func (p *GatherFacts) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.investigateHost)
}

func (p *GatherFacts) investigateHost(h *cluster.Host) error {
	log.Infof("%s: investigating host", h)

	if h.Configurer.ServiceIsRunning("k0s") {
		h.Metadata.K0sRunning = true
		log.Infof("%s: k0s service is running", h)
	}

	if h.Role == "server" && h.Configurer.FileExist(h.Configurer.K0sJoinTokenPath()) {
		token, err := h.Configurer.ReadFile(h.Configurer.K0sJoinTokenPath())
		if token != "" && err == nil {
			log.Infof("%s: found an existing controller token", h)
			p.Config.Spec.K0s.Metadata.ControllerToken = token
		}
	}

	if h.Role == "worker" && h.Configurer.FileExist(h.Configurer.K0sJoinTokenPath()) {
		token, err := h.Configurer.ReadFile(h.Configurer.K0sJoinTokenPath())
		if token != "" && err == nil {
			log.Infof("%s: found an existing worker token", h)
			p.Config.Spec.K0s.Metadata.WorkerToken = token
		}
	}

	if output, err := h.ExecOutput("k0s version"); err == nil {
		h.Metadata.K0sVersion = strings.TrimPrefix(output, "v")
		log.Infof("%s: has k0s binary version %s", h, h.Metadata.K0sVersion)
	}

	output, err := h.Configurer.Arch()
	if err != nil {
		return err
	}
	h.Metadata.Arch = output
	log.Infof("%s: architecture is %s", h, h.Metadata.Arch)

	return nil
}
