package phase

import (
	"fmt"

	"github.com/Masterminds/semver"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	log "github.com/sirupsen/logrus"
)

// Reset uninstalls k0s from the hosts
type Reset struct {
	GenericPhase
	hosts cluster.Hosts
}

// Title for the phase
func (p *Reset) Title() string {
	return "Reset hosts"
}

// Prepare the phase
func (p *Reset) Prepare(config *config.Cluster) error {
	p.Config = config
	var hosts cluster.Hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.Metadata.K0sBinaryVersion != ""
	})
	min, _ := semver.NewVersion("0.11.0")

	for _, h := range hosts {
		running, err := semver.NewVersion(h.Metadata.K0sBinaryVersion)
		if err != nil {
			return err
		}

		if running.LessThan(min) {
			return fmt.Errorf("reset is only supported on k0s >= 0.11.0")
		}
	}

	p.hosts = hosts

	return nil
}

// Run the phase
func (p *Reset) Run() error {
	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		if h.Configurer.ServiceIsRunning(h, h.K0sServiceName()) {
			log.Infof("%s: stopping k0s", h)
			if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
				return err
			}
		}

		log.Infof("%s: running k0s reset", h)
		return h.Exec(h.Configurer.K0sCmdf("reset"))
	})
}
