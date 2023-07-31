package phase

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
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
		// Nothing to upload
		if h.UploadBinaryPath == "" {
			return false
		}

		// Nothing to upload
		if h.Reset {
			return false
		}

		// The version is already correct
		if p.Config.Spec.K0s.VersionEquals(h.Metadata.K0sBinaryVersion) {
			return false
		}

		if !h.FileChanged(h.UploadBinaryPath, h.Configurer.K0sBinaryPath()) {
			return false
		}

		return true
	})
	return nil
}

// ShouldRun is true when there are hosts that need binary uploading
func (p *UploadBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UploadBinaries) Run() error {
	return p.parallelDoUpload(p.hosts, p.uploadBinary)
}

func (p *UploadBinaries) uploadBinary(h *cluster.Host) error {
	tmp, err := h.Configurer.TempFile(h)
	if err != nil {
		return fmt.Errorf("failed to create tempfile %w", err)
	}

	stat, err := os.Stat(h.UploadBinaryPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", h.UploadBinaryPath, err)
	}

	log.Infof("%s: uploading k0s binary from %s", h, h.UploadBinaryPath)
	if err := h.Upload(h.UploadBinaryPath, tmp); err != nil {
		return fmt.Errorf("upload k0s binary: %w", err)
	}

	if err := h.Configurer.Touch(h, tmp, stat.ModTime()); err != nil {
		return fmt.Errorf("failed to touch %s: %w", tmp, err)
	}

	h.Metadata.K0sBinaryTempFile = tmp

	return nil
}
