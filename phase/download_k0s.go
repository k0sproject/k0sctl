package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// DownloadK0s connects to each of the hosts
type DownloadK0s struct {
	GenericPhase
	hosts cluster.Hosts
}

func (p *DownloadK0s) Title() string {
	return "Download K0s"
}

func (p *DownloadK0s) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.Metadata.K0sVersion != p.Config.Spec.K0s.Version
	})
	return nil
}

func (p *DownloadK0s) ShouldRun() bool {
	return len(p.hosts) > 0
}

func (p *DownloadK0s) Run() error {
	return p.hosts.ParallelEach(p.downloadK0s)
}

func (p *DownloadK0s) downloadK0s(h *cluster.Host) error {
	log.Infof("%s: downloading k0s", h)
	return h.Configurer.RunK0sDownloader(p.Config.Spec.K0s.Version)
}
