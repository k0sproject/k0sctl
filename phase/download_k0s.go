package phase

import (
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
	return "Download K0s"
}

// Prepare the phase
func (p *DownloadK0s) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.Metadata.K0sVersion != p.Config.Spec.K0s.Version
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
	log.Infof("%s: downloading k0s", h)
	return h.Configurer.RunK0sDownloader(p.Config.Spec.K0s.Version)
}
