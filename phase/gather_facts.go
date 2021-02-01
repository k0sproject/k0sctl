package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
)

// Note: Passwordless sudo has not yet been confirmed when this runs

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
	p.IncProp(h.Role)

	output, err := h.Configurer.Arch(h)
	if err != nil {
		return err
	}
	h.Metadata.Arch = output
	p.IncProp(h.Metadata.Arch)

	h.Metadata.Hostname = h.Configurer.Hostname(h)

	return nil
}
