package phase

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
)

// InjectARMFixes implements a phase which fixes arm quirks (must run before uploadfiles)
type InjectARMFixes struct {
	GenericPhase

	hosts cluster.Hosts
}

// Title for the phase
func (p *InjectARMFixes) Title() string {
	return "Fix ARM quirks"
}

// Prepare the phase
func (p *InjectARMFixes) Prepare(config *config.Cluster) error {
	p.Config = config

	hosts := p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		if h.Role == "worker" {
			return false
		}
		arch := h.Metadata.Arch
		return arch == "arm" || arch == "arm64"
	})
	p.hosts = hosts

	return nil
}

// ShouldRun is true when there are workers
func (p *InjectARMFixes) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *InjectARMFixes) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.FixARMQuirks)
}

func (p *InjectARMFixes) FixARMQuirks(h *cluster.Host) error {
	log.Infof("Host %s uses arm architecture; setting ETCD_UNSUPPORTED_ARCH=%s", h.Metadata.Hostname, h.Metadata.Arch)

	systemdOverride := fmt.Sprintf(`[Service]
Environment=ETCD_UNSUPPORTED_ARCH=%s
`, h.Metadata.Arch)

	return h.Configurer.WriteFile(h, "/etc/systemd/system/k0scontroller.service.d/override.conf", systemdOverride, "0644")
}
