package phase

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// UploadK0s uploads k0s binaries from localhost to target
type UploadK0s struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *UploadK0s) Title() string {
	return "Upload k0s binaries to hosts"
}

// Prepare the phase
func (p *UploadK0s) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		// Nothing to upload
		if h.UploadBinaryPath == "" {
			return false
		}

		// No need to upload, host is going to be reset
		if h.Reset {
			return false
		}

		if !p.Config.Spec.K0s.Version.Equal(h.Metadata.K0sBinaryVersion) {
			log.Debugf("%s: k0s version on host is '%s'", h, h.Metadata.K0sBinaryVersion)
			return true
		}

		// If the file has been changed compared to local, re-upload and replace
		return h.FileChanged(h.UploadBinaryPath, h.K0sInstallLocation())
	})
	return nil
}

// ShouldRun is true when there are hosts that need binary uploading
func (p *UploadK0s) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UploadK0s) Run(ctx context.Context) error {
	return p.parallelDoUpload(ctx, p.hosts, p.uploadBinary)
}

func (p *UploadK0s) uploadBinary(_ context.Context, h *cluster.Host) error {
	tmp := h.K0sInstallLocation() + ".tmp." + strconv.Itoa(int(time.Now().UnixNano()))

	stat, err := os.Stat(h.UploadBinaryPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", h.UploadBinaryPath, err)
	}

	log.Infof("%s: uploading k0s binary from %s to %s", h, h.UploadBinaryPath, tmp)
	if err := h.Upload(h.UploadBinaryPath, tmp, 0o600, exec.Sudo(h), exec.LogError(true)); err != nil {
		return fmt.Errorf("upload k0s binary: %w", err)
	}

	if err := h.Configurer.Touch(h, tmp, stat.ModTime(), exec.Sudo(h)); err != nil {
		return fmt.Errorf("failed to touch %s: %w", tmp, err)
	}
	if err := h.Execf(`chmod +x "%s"`, tmp, exec.Sudo(h)); err != nil {
		log.Warnf("%s: failed to chmod k0s temp binary: %v", h, err.Error())
	}

	h.Metadata.K0sBinaryTempFile = tmp

	return nil
}

// Cleanup removes the binary temp file if it wasn't used
func (p *UploadK0s) CleanUp() {
	_ = p.parallelDo(context.Background(), p.hosts, func(_ context.Context, h *cluster.Host) error {
		if h.Metadata.K0sBinaryTempFile != "" {
			_ = h.Configurer.DeleteFile(h, h.Metadata.K0sBinaryTempFile)
		}
		return nil
	})
}
