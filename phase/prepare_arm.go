package phase

import (
	"strings"

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
		return h.Role != "worker" && (strings.HasPrefix(arch, "arm") || strings.HasPrefix(arch, "aarch"))
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
	var arch string
	arch := "arm64"
	switch h.Metadata.Arch {
	case "aarch32", "arm32", "armv7l", "armhfp", "arm-32":
		arch = "arm32"
	default:
		arch = "arm64"
	}

	log.Warnf("%s: enabling ETCD_UNSUPPORTED_ARCH=%s override - you may encounter problems with etcd", h, arch)
	h.Environment["ETCD_UNSUPPORTED_ARCH"] = arch
}
