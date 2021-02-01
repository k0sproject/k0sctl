package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
)

// Disconnect disconnects from the hosts
type Disconnect struct {
	GenericPhase
}

// Title for the phase
func (p *Disconnect) Title() string {
	return "Disconnect from hosts"
}

// Run the phase
func (p *Disconnect) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(func(h *cluster.Host) error {
		h.Disconnect()
		return nil
	})
}
