package phase

import (
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
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
	log.Infof("%s: investigating host", h)
	p.IncProp(h.Role)

	output, err := h.Configurer.Arch(h)
	if err != nil {
		return err
	}
	h.Metadata.Arch = output
	p.IncProp(h.Metadata.Arch)
	log.Infof("%s: cpu architecture is %s", h, h.Metadata.Arch)

	h.Metadata.Hostname = h.Configurer.Hostname(h)

	if h.IsController() {
		if h.PrivateAddress == "" {
			if h.PrivateInterface == "" {
				if iface, err := h.Configurer.PrivateInterface(h); err == nil {
					h.PrivateInterface = iface
					log.Infof("%s: discovered %s as private interface", h, iface)
				}
			}

			if h.PrivateInterface != "" {
				if addr, err := h.Configurer.PrivateAddress(h, h.PrivateInterface, h.Address()); err == nil {
					h.PrivateAddress = addr
					log.Infof("%s: discovered %s as private address", h, addr)
				}
			}
		}
	}

	return nil
}
