package phase

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
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
		_ = h.Exec("ps -ef", exec.Sudo(h), exec.StreamOutput())
		h.Disconnect()
		return nil
	})
}
