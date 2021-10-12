package phase

import (
	"fmt"

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
	p.IncProp(h.Role)

	output, err := h.Configurer.Arch(h)
	if err != nil {
		return err
	}
	h.Metadata.Arch = output
	p.IncProp(h.Metadata.Arch)

	extra := h.InstallFlags.GetValue("--kubelet-extra-args")
	if extra != "" {
		ef := cluster.Flags{extra}
		if over := ef.GetValue("--hostname-override"); over != "" {
			if h.HostnameOverride != over {
				return fmt.Errorf("hostname and installFlags kubelet-extra-args hostname-override mismatch, only define either one")
			}
			h.HostnameOverride = over
		}
	}

	if h.HostnameOverride != "" {
		log.Infof("%s: using %s from configuration as hostname", h, h.HostnameOverride)
		h.Metadata.Hostname = h.HostnameOverride
	} else {
		h.Metadata.Hostname = h.Configurer.Hostname(h)
		log.Infof("%s: using %s as hostname", h, h.Metadata.Hostname)
	}

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

	return nil
}
