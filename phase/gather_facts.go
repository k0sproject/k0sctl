package phase

import (
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// Note: Passwordless sudo has not yet been confirmed when this runs

// GatherFacts gathers information about hosts, such as if k0s is already up and running
type GatherFacts struct {
	GenericPhase
	SkipMachineIDs bool
}

// K0s doesn't rely on unique machine IDs anymore since v1.30.
var uniqueMachineIDVersion = version.MustConstraint("< v1.30")

// Title for the phase
func (p *GatherFacts) Title() string {
	return "Gather host facts"
}

// Run the phase
func (p *GatherFacts) Run() error {
	return p.parallelDo(p.Config.Spec.Hosts, p.investigateHost)
}

func (p *GatherFacts) investigateHost(h *cluster.Host) error {
	p.IncProp(h.Role)

	output, err := h.Configurer.Arch(h)
	if err != nil {
		return err
	}
	h.Metadata.Arch = output

	if !p.SkipMachineIDs && uniqueMachineIDVersion.Check(p.Config.Spec.K0s.Version) {
		id, err := h.Configurer.MachineID(h)
		if err != nil {
			return err
		}
		h.Metadata.MachineID = id
	}

	p.IncProp(h.Metadata.Arch)

	if extra := h.InstallFlags.GetValue("--kubelet-extra-args"); extra != "" {
		ef := cluster.Flags{extra}
		if over := ef.GetValue("--hostname-override"); over != "" {
			if h.HostnameOverride != "" && h.HostnameOverride != over {
				return fmt.Errorf("hostname and installFlags kubelet-extra-args hostname-override mismatch, only define either one")
			}
			h.HostnameOverride = over
		}
	}

	if h.HostnameOverride != "" {
		log.Infof("%s: using %s from configuration as hostname", h, h.HostnameOverride)
		h.Metadata.Hostname = h.HostnameOverride
	} else {
		n := h.Configurer.Hostname(h)
		if n == "" {
			return fmt.Errorf("%s: failed to resolve a hostname", h)
		}
		h.Metadata.Hostname = n
		log.Infof("%s: using %s as hostname", h, n)
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
