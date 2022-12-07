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
		// Nothing to upload
		if h.UploadBinary {
			return false
		}

		// Nothing to upload
		if h.Reset {
			return false
		}

		// Upgrade is handled separately (k0s stopped, binary uploaded, k0s restarted)
		if h.Metadata.NeedsUpgrade {
			return false
		}

		// The version is already correct
		if h.Metadata.K0sBinaryVersion == p.Config.Spec.K0s.Version {
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
		return err
	}

	log.Infof("%s: downloading k0s %s", h, targetVersion)
	if err := h.Configurer.DownloadK0s(h, targetVersion, h.Metadata.Arch); err != nil {
		return err
	}

	downloadedVersion, err := h.Configurer.K0sBinaryVersion(h)
	if err != nil {
		if err := h.Configurer.DeleteFile(h, h.Configurer.K0sBinaryPath()); err != nil {
			log.Warnf("%s: failed to remove %s: %s", h, h.Configurer.K0sBinaryPath(), err.Error())
		}
		return fmt.Errorf("failed to get downloaded k0s binary version: %w", err)
	}
	if !targetVersion.Equal(downloadedVersion) {
		return fmt.Errorf("downloaded k0s binary version is %s not %s", downloadedVersion, targetVersion)
	}

	h.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version

	return nil
}
