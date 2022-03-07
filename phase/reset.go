package phase

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
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
func (p *Reset) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	var hosts cluster.Hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.Metadata.K0sBinaryVersion != ""
	})
	c, _ := semver.NewConstraint("< 0.11.0-rc1")

	for _, h := range hosts {
		running, err := semver.NewVersion(h.Metadata.K0sBinaryVersion)
		if err != nil {
			return err
		}

		if c.Check(running) {
			return fmt.Errorf("reset is only supported on k0s >= 0.11.0-rc1")
		}
	}

	p.hosts = hosts

	return nil
}

// Run the phase
func (p *Reset) Run() error {
	return p.hosts.ParallelEach(func(h *cluster.Host) error {
		log.Infof("%s: cleaning up service environment", h)
		if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
			return err
		}

		if h.Configurer.ServiceIsRunning(h, h.K0sServiceName()) {
			log.Infof("%s: stopping k0s", h)
			if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
				return err
			}
			log.Infof("%s: waiting for k0s to stop", h)
			if err := h.WaitK0sServiceStopped(); err != nil {
				return err
			}
		}

		log.Infof("%s: running k0s reset", h)
		out, err := h.ExecOutput(h.Configurer.K0sCmdf("reset"), exec.Sudo(h))
		c, _ := semver.NewConstraint("<= 1.22.3+k0s.0")
		running, _ := semver.NewVersion(h.Metadata.K0sBinaryVersion)
		if err != nil {
			log.Warnf("%s: k0s reported failure: %v", h, err)
			if c.Check(running) && strings.Contains(out, "k0s cleanup operations done") {
				return nil
			}
		}

		if err := h.Configurer.DeleteFile(h, h.Configurer.K0sConfigPath()); err != nil {
			log.Warnf("%s: failed to remove existing configuration %s: %s", h, h.Configurer.K0sConfigPath(), err)
		}

		return nil
	})
}
