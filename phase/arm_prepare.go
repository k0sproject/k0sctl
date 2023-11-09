package phase

import (
	"strings"

	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

var etcdSupportedArchArm64Since = version.MustConstraint(">= v1.22.1+k0s.0")

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
		if h.Reset {
			return false
		}

		if h.Role == "worker" {
			return false
		}

		arch := h.Metadata.Arch

		if !strings.HasPrefix(arch, "arm") && !strings.HasPrefix(arch, "aarch") {
			return false
		}

		if strings.HasSuffix(arch, "64") {
			// 64-bit arm is supported on etcd 3.5.0+ which is included in k0s v1.22.1+k0s.0 and newer
			if etcdSupportedArchArm64Since.Check(p.Config.Spec.K0s.Version) {
				return false
			}
		}

		return true
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
