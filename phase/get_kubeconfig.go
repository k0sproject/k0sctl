package phase

import (
	"fmt"

	"github.com/k0sproject/rig/exec"
)

// GetKubeconfig is a phase to get and dump the admin kubeconfig
type GetKubeconfig struct {
	GenericPhase
	APIAddress string
}

// Title for the phase
func (p *GetKubeconfig) Title() string {
	return "Get admin kubeconfig"
}

func (p *GetKubeconfig) DryRun() error {
	p.DryMsg(p.Config.Spec.Hosts.Controllers()[0], "get admin kubeconfig")
	return nil
}

// Run the phase
func (p *GetKubeconfig) Run() error {
	h := p.Config.Spec.K0sLeader()

	output, err := h.ExecOutput(h.Configurer.K0sCmdf("kubeconfig admin"), exec.Sudo(h))
	if err != nil {
		return fmt.Errorf("read kubeconfig from host: %w", err)
	}

	p.Config.Metadata.Kubeconfig = output

	return nil
}
