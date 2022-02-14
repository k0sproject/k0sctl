package phase

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// UploadBinaries uploads k0s binaries from localhost to target
type UploadBinaries struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *UploadBinaries) Title() string {
	return "Upload k0s binaries to hosts"
}

// Prepare the phase
func (p *UploadBinaries) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.UploadBinaryPath != "" && h.Metadata.K0sBinaryVersion != p.Config.Spec.K0s.Version && !h.Metadata.NeedsUpgrade
	})
	return nil
}

// ShouldRun is true when there are hosts that need binary uploading
func (p *UploadBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UploadBinaries) Run() error {
	return p.hosts.ParallelEach(p.uploadBinary)
}

func (p *UploadBinaries) uploadBinary(h *cluster.Host) error {
	log.Infof("%s: uploading k0s binary from %s", h, h.UploadBinaryPath)
	if err := h.Upload(h.UploadBinaryPath, h.Configurer.K0sBinaryPath(), exec.Sudo(h)); err != nil {
		return err
	}

	if err := h.Configurer.Chmod(h, h.Configurer.K0sBinaryPath(), "0700", exec.Sudo(h)); err != nil {
		return err
	}

	h.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version

	return nil
}
