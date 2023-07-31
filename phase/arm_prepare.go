package phase

import (
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
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
func (p *PrepareArm) Prepare(config *v1beta1.Cluster) error {
	p.Config = config

	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		arch := h.Metadata.Arch
		return h.Role != "worker" && (strings.HasPrefix(arch, "arm") || strings.HasPrefix(arch, "aarch")) && !strings.HasSuffix(arch, "64")
	})

	return nil
}

// ShouldRun is true when there are arm controllers
func (p *PrepareArm) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *PrepareArm) Run() error {
	return p.parallelDo(p.hosts, p.etcdUnsupportedArch)
}

func (p *PrepareArm) etcdUnsupportedArch(h *cluster.Host) error {
	log.Warnf("%s: enabling ETCD_UNSUPPORTED_ARCH=%s override - you may encounter problems with etcd", h, h.Metadata.Arch)
	h.Environment["ETCD_UNSUPPORTED_ARCH"] = h.Metadata.Arch

	return nil
}
