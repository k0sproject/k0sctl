package phase

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// Disconnect disconnects from the hosts
type Disconnect struct {
	GenericPhase
}

// Title for the phase
func (p *Disconnect) Title() string {
	return "Disconnect from hosts"
}

// DryRun cleans up the temporary k0s binary from the hosts
func (p *Disconnect) DryRun() error {
	_ = p.Config.Spec.Hosts.ParallelEach(func(h *cluster.Host) error {
		if h.Metadata.K0sBinaryTempFile != "" && h.Configurer.FileExist(h, h.Metadata.K0sBinaryTempFile) {
			_ = h.Configurer.DeleteFile(h, h.Metadata.K0sBinaryTempFile)
		}
		return nil
	})

	return p.Run()
}

// Run the phase
func (p *Disconnect) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(func(h *cluster.Host) error {
		h.Disconnect()
		return nil
	})
}
