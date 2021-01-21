package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// UploadBinaries uploads k0s binaries from localhost to target
type UploadBinaries struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *UploadBinaries) Title() string {
	return "Upload binaries"
}

// Prepare the phase
func (p *UploadBinaries) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.K0sBinaryPath != "" && h.Metadata.K0sVersion != p.Config.Spec.K0s.Version
	})
	return nil
}

// ShouldRun is true when there are hosts that need binary uploading
func (p *UploadBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UploadBinaries) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.uploadBinary)
}

func (p *UploadBinaries) uploadBinary(h *cluster.Host) error {
	log.Infof("%s: uploading k0s binary from %s", h, h.K0sBinaryPath)
	if err := h.Upload(h.K0sBinaryPath, h.Configurer.K0sBinaryPath()); err != nil {
		return err
	}

	if err := h.Configurer.Chmod(h.Configurer.K0sBinaryPath(), "0700"); err != nil {
		return err
	}

	h.Metadata.K0sVersion = p.Config.Spec.K0s.Version

	return nil
}
