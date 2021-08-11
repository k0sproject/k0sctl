package phase

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
)

// PrepareArm implements a phase which fixes arm quirks
type PrepareArm struct {
	GenericPhase

	hosts cluster.Hosts
}

// Title for the phase
func (p *PrepareArm) Title() string {
	return "Prepare ARM nodes"
}

// Prepare the phase
func (p *PrepareArm) Prepare(config *config.Cluster) error {
	p.Config = config

	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		arch := h.Metadata.Arch
		return h.Role != "worker" && (arch == "arm" || arch == "arm64")
	})

	return nil
}

// ShouldRun is true when there are arm controllers
func (p *PrepareArm) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *PrepareArm) Run() error {
	return p.hosts.ParallelEach(p.etcdUnsupportedArch)
}

func (p *PrepareArm) etcdUnsupportedArch(h *cluster.Host) error {
	log.Infof("%s: enabling ETCD_UNSUPPORTED_ARCH=%s override", h, h.Metadata.Arch)

	return h.Configurer.WriteFile(
		h,
		"/etc/systemd/system/k0scontroller.service.d/override.conf",
		fmt.Sprintf("[Service]\nEnvironment=ETCD_UNSUPPORTED_ARCH=%s\n", h.Metadata.Arch),
		"0644",
	)
}
