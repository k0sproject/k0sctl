package phases

// import (
// 	api "github.com/k0sproject/k0sctl/config"
// 	"github.com/k0sproject/k0sctl/phase"
// 	log "github.com/sirupsen/logrus"
// 	"gopkg.in/yaml.v2"
// )

// // ConfigureK0s phase
// type ConfigureK0s struct {
// 	phase.Analytics
// 	BasicPhase
// }

// // Title for the phase
// func (p *ConfigureK0s) Title() string {
// 	return "Configure K0s on Hosts"
// }

// // Run ...
// func (p *ConfigureK0s) Run() error {
// 	return RunParallelOnHosts(p.Config.Spec.Hosts, p.Config, p.writeConfig)
// }

// func (p *ConfigureK0s) writeConfig(h *api.Host, c *api.ClusterConfig) error {
// 	if h.Role == "server" {
// 		log.Infof("%s: writing K0s config", h)

// 		output, err := yaml.Marshal(c.Spec.K0s.Config)
// 		if err != nil {
// 			return err
// 		}
// 		return h.Configurer.WriteFile(h.Configurer.K0sConfigPath(), string(output), "0700")
// 	}

// 	return nil
// }
