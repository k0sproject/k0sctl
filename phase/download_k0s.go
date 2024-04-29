package phase

import (
	"fmt"
	"strconv"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
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
		if p.Config.Spec.K0s.Version.Equal(h.Metadata.K0sBinaryVersion) {
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
	tmp := h.Configurer.K0sBinaryPath() + ".tmp." + strconv.Itoa(int(time.Now().UnixNano()))

	log.Infof("%s: downloading k0s %s", h, p.Config.Spec.K0s.Version)
	if h.K0sDownloadURL != "" {
		expandedURL := h.ExpandTokens(h.K0sDownloadURL, p.Config.Spec.K0s.Version)
		log.Infof("%s: downloading k0s binary from %s", h, expandedURL)
		if err := h.Configurer.DownloadURL(h, expandedURL, tmp); err != nil {
			return fmt.Errorf("failed to download k0s binary: %w", err)
		}
	} else if err := h.Configurer.DownloadK0s(h, tmp, p.Config.Spec.K0s.Version, h.Metadata.Arch); err != nil {
		return err
	}

	if err := h.Execf(`chmod +x "%s"`, tmp, exec.Sudo(h)); err != nil {
		log.Warnf("%s: failed to chmod k0s temp binary: %v", h, err.Error())
	}

	h.Metadata.K0sBinaryTempFile = tmp

	return nil
}
