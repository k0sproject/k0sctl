package phase

import (
	"context"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
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
		if h.UseExistingK0s {
			return false
		}
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
func (p *DownloadK0s) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.hosts, p.downloadK0s)
}

func (p *DownloadK0s) downloadK0s(_ context.Context, h *cluster.Host) error {
	ts := strconv.Itoa(int(time.Now().UnixNano()))
	bin := h.K0sInstallLocation()
	tmp := bin + ".tmp." + ts
	if h.IsConnected() && h.IsWindows() {
		if strings.HasSuffix(strings.ToLower(bin), ".exe") {
			tmp = strings.TrimSuffix(bin, ".exe") + ".tmp." + ts + ".exe"
		}
	}

	log.Infof("%s: downloading k0s %s", h, p.Config.Spec.K0s.Version)
	url, err := h.K0sDownloadURL(p.Config.Spec.K0s.Version)
	if err != nil {
		return fmt.Errorf("failed to determine k0s download url: %w", err)
	}
	// Ensure directory exists
	log.Debugf("%s: ensuring k0s install directory exists %s", h, h.Configurer.Dir(tmp))
	if err := h.SudoFsys().MkDirAll(h.Configurer.Dir(tmp), fs.FileMode(0o755)); err != nil {
		return fmt.Errorf("failed to create k0s install directory: %w", err)
	}
	if err := h.Configurer.DownloadURL(h, url, tmp, exec.Sudo(h)); err != nil {
		return fmt.Errorf("failed to download k0s binary: %w", err)
	}
	h.Metadata.K0sBinaryTempFile = tmp

	if h.IsWindows() {
		return nil
	}

	if err := chmodWithMode(h, tmp, fs.FileMode(0o755)); err != nil {
		log.Debugf("%s: chmod %s failed: %v", h, tmp, err)
	}

	return nil
}

// Cleanup removes the binary temp file if it wasn't used
func (p *DownloadK0s) CleanUp() {
	_ = p.parallelDo(context.Background(), p.hosts, func(_ context.Context, h *cluster.Host) error {
		if h.Metadata.K0sBinaryTempFile != "" {
			_ = h.Configurer.DeleteFile(h, h.Metadata.K0sBinaryTempFile)
		}
		return nil
	})
}
