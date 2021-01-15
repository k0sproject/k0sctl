package phases

// import (
// 	"github.com/Mirantis/mcc/pkg/phase"
// 	"github.com/Mirantis/mcc/pkg/product/k0s/api"

// 	retry "github.com/avast/retry-go"
// 	log "github.com/sirupsen/logrus"
// )

// // PrepareHost phase implementation does all the prep work we need for the hosts
// type PrepareHost struct {
// 	phase.Analytics
// 	BasicPhase
// }

// // Title for the phase
// func (p *PrepareHost) Title() string {
// 	return "Prepare hosts"
// }

// // Run does all the prep work on the hosts in parallel
// func (p *PrepareHost) Run() error {
// 	err := RunParallelOnHosts(p.Config.Spec.Hosts, p.Config, p.updateEnvironment)
// 	if err != nil {
// 		return err
// 	}

// 	err = RunParallelOnHosts(p.Config.Spec.Hosts, p.Config, p.installBasePackages)
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

// func (p *PrepareHost) installBasePackages(h *api.Host, c *api.ClusterConfig) error {
// 	err := retry.Do(
// 		func() error {
// 			log.Infof("%s: installing base packages", h)
// 			err := h.Configurer.InstallK0sBasePackages()

// 			return err
// 		},
// 	)
// 	if err != nil {
// 		log.Errorf("%s: failed to install base packages -> %s", h, err.Error())
// 		return err
// 	}

// 	log.Infof("%s: base packages installed", h)
// 	return nil
// }

// func (p *PrepareHost) updateEnvironment(h *api.Host, c *api.ClusterConfig) error {
// 	if len(h.Environment) > 0 {
// 		log.Infof("%s: updating environment", h)
// 		return h.Configurer.UpdateEnvironment(h.Environment)
// 	}

// 	log.Debugf("%s: no environment variables specified for the host", h)
// 	return nil
// }
