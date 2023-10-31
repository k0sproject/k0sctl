package phase

import (
	"fmt"

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

		return h.Metadata.K0sBinaryTempFile != ""
	})
	return nil
}

// ShouldRun is true when the phase should be run
func (p *InstallBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *InstallBinaries) Run() error {
	return p.parallelDo(p.hosts, p.installBinary)
}

func (p *InstallBinaries) installBinary(h *cluster.Host) error {
	if err := h.UpdateK0sBinary(h.Metadata.K0sBinaryTempFile, p.Config.Spec.K0s.Version); err != nil {
		return fmt.Errorf("failed to install k0s binary: %w", err)
	}

	return nil
}

func (p *InstallBinaries) CleanUp() {
	err := p.parallelDo(p.hosts, func(h *cluster.Host) error {
		if h.Metadata.K0sBinaryTempFile == "" {
			return nil
		}
		logrus.Infof("%s: cleaning up k0s binary tempfile", h)
		if err := h.Configurer.DeleteFile(h, h.Metadata.K0sBinaryTempFile); err != nil {
			return fmt.Errorf("clean up tempfile: %w", err)
		}
		return nil
	})

	if err != nil {
		logrus.Debugf("failed to clean up tempfiles: %v", err)
	}
}
