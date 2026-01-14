package phase

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/sirupsen/logrus"
)

// InstallBinaries installs the k0s binaries from the temp location of UploadBinaries or InstallBinaries
type InstallBinaries struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *InstallBinaries) Title() string {
	return "Install k0s binaries on hosts"
}

// Prepare the phase
func (p *InstallBinaries) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		if h.Reset && h.Metadata.K0sBinaryVersion != nil {
			return false
		}

		// Upgrade is handled in UpgradeControllers/UpgradeWorkers phases
		if h.Metadata.NeedsUpgrade {
			return false
		}

		if h.UseExistingK0s {
			return false
		}
		return h.Metadata.K0sBinaryTempFile != ""
	})
	return nil
}

// ShouldRun is true when the phase should be run
func (p *InstallBinaries) ShouldRun() bool {
	return len(p.hosts) > 0 || !p.IsWet()
}

// DryRun reports what would happen if Run is called.
func (p *InstallBinaries) DryRun() error {
	return p.parallelDo(
		context.Background(),
		p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool { return h.Metadata.K0sBinaryTempFile != "" }),
		func(_ context.Context, h *cluster.Host) error {
			p.DryMsgf(h, "install k0s %s binary from %s to %s", p.Config.Spec.K0s.Version, h.Metadata.K0sBinaryTempFile, h.K0sInstallLocation())
			if err := chmodWithMode(h, h.Metadata.K0sBinaryTempFile, fs.FileMode(0o755)); err != nil {
				logrus.Warnf("%s: failed to chmod k0s temp binary for dry-run: %s", h, err.Error())
			}
			h.Configurer.SetPath("K0sBinaryPath", h.Metadata.K0sBinaryTempFile)
			h.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version
			return nil
		},
	)
}

// Run the phase
func (p *InstallBinaries) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.hosts, p.installBinary)
}

func (p *InstallBinaries) installBinary(_ context.Context, h *cluster.Host) error {
	logrus.Debugf("%s: installing k0s binary from tempfile %s to %s", h, h.Metadata.K0sBinaryTempFile, h.K0sInstallLocation())
	if err := h.UpdateK0sBinary(h.Metadata.K0sBinaryTempFile, p.Config.Spec.K0s.Version); err != nil {
		return fmt.Errorf("failed to install k0s binary: %w", err)
	}
	h.Metadata.K0sBinaryTempFile = ""

	return nil
}

func (p *InstallBinaries) CleanUp() {
	err := p.parallelDo(context.Background(), p.hosts, func(_ context.Context, h *cluster.Host) error {
		if h.Metadata.K0sBinaryTempFile == "" {
			return nil
		}
		logrus.Infof("%s: cleaning up k0s binary tempfile", h)
		_ = h.Configurer.DeleteFile(h, h.Metadata.K0sBinaryTempFile)
		return nil
	})
	if err != nil {
		logrus.Debugf("failed to clean up tempfiles: %v", err)
	}
}
