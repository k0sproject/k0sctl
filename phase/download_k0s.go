package phase

import (
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// DownloadK0s performs k0s online download on the hosts
type DownloadK0s struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *DownloadK0s) Title() string {
	return "Download k0s on hosts"
}

// Prepare the phase
func (p *DownloadK0s) Prepare(config *v1beta1.Cluster) error {
	p.Config = config

	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		// Nothing to download
		if h.UploadBinary {
			return false
		}

		// No need to download, host is going to be reset
		if h.Reset {
			return false
		}

		// The version on host is already same as the target version
		if p.Config.Spec.K0s.VersionEqual(h.Metadata.K0sBinaryVersion) {
			log.Debugf("%s: k0s version on target host is already %s", h, h.Metadata.K0sBinaryVersion)
			return false
		}

		return true
	})

	return nil
}

// ShouldRun is true when the phase should be run
func (p *DownloadK0s) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *DownloadK0s) Run() error {
	return p.parallelDo(p.hosts, p.downloadK0s)
}

func (p *DownloadK0s) downloadK0s(h *cluster.Host) error {
	targetVersion, err := version.NewVersion(p.Config.Spec.K0s.Version)
	if err != nil {
		return fmt.Errorf("parse k0s version: %w", err)
	}

	tmp, err := h.Configurer.TempFile(h)
	if err != nil {
		return fmt.Errorf("failed to create tempfile %w", err)
	}

	log.Infof("%s: downloading k0s %s", h, targetVersion)
	if err := h.Configurer.DownloadK0s(h, tmp, targetVersion, h.Metadata.Arch); err != nil {
		return err
	}

	h.Metadata.K0sBinaryTempFile = tmp

	return nil
}
