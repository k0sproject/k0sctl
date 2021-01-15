package phases

// import (
// 	"github.com/Mirantis/mcc/pkg/phase"

// 	"github.com/Mirantis/mcc/pkg/product/k0s/api"
// )

// // RunK0sDownloader phase implementation to collect facts (OS, version etc.) from hosts
// type RunK0sDownloader struct {
// 	phase.Analytics
// 	BasicPhase

// 	hosts []*api.Host
// }

// // Title for the phase
// func (p *RunK0sDownloader) Title() string {
// 	return "Run K0s Downloader"
// }

// func (p *RunK0sDownloader) Prepare(config interface{}) error {
// 	p.Config = config.(*api.ClusterConfig)

// 	for _, h := range p.Config.Spec.Hosts {
// 		if h.K0sBinary != "" || h.Metadata.K0sVersion == p.Config.Spec.K0s.Version {
// 			continue
// 		}
// 		p.hosts = append(p.hosts, h)
// 	}

// 	return nil
// }

// func (p *RunK0sDownloader) ShouldRun() bool {
// 	return len(p.hosts) > 0
// }

// // Run collect all the facts from hosts in parallel
// func (p *RunK0sDownloader) Run() error {
// 	return RunParallelOnHosts(p.Config.Spec.Hosts, p.Config, p.runInstaller)
// }

// func (p *RunK0sDownloader) runInstaller(h *api.Host, c *api.ClusterConfig) error {
// 	return h.Configurer.RunK0sDownloader(c.Spec.K0s.Version)
// }
