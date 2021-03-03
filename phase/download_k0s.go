package phase

import (
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
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
func (p *DownloadK0s) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.Metadata.K0sBinaryVersion != p.Config.Spec.K0s.Version && !h.Metadata.NeedsUpgrade
	})
	return nil
}

// ShouldRun is true when the phase should be run
func (p *DownloadK0s) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *DownloadK0s) Run() error {
	return p.hosts.ParallelEach(p.downloadK0s)
}

func (p *DownloadK0s) downloadK0s(h *cluster.Host) error {
	target := p.Config.Spec.K0s.Version
	log.Infof("%s: downloading k0s %s", h, target)
	if err := h.Configurer.DownloadK0s(h, target, h.Metadata.Arch); err != nil {
		return err
	}

	output, err := h.ExecOutput(h.Configurer.K0sCmdf("version"))
	if err != nil {
		if err := h.Configurer.DeleteFile(h, h.Configurer.K0sBinaryPath()); err != nil {
			log.Warnf("%s: failed to remove %s: %s", h, h.Configurer.K0sBinaryPath(), err.Error())
		}
		return fmt.Errorf("downloaded k0s binary is invalid: %s", err.Error())
	}
	output = strings.TrimPrefix(output, "v")
	if output != target {
		return fmt.Errorf("downloaded k0s binary version is %s not %s", output, target)
	}

	h.Metadata.K0sBinaryVersion = target

	return nil
}
