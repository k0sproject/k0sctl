package phase

import (
	"strings"

	"github.com/Masterminds/semver"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// UninstallControllers uninstalls k0s from the controllers
type UninstallControllers struct {
	GenericPhase

	NoDrain bool

	hosts  cluster.Hosts
	leader *cluster.Host
}

// Title for the phase
func (p *UninstallControllers) Title() string {
	return "Uninstall controllers"
}

// Prepare the phase
func (p *UninstallControllers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	log.Debugf("%d controllers in total", len(controllers))
	p.hosts = controllers.Filter(func(h *cluster.Host) bool {
		return h.Uninstall
	})
	log.Debugf("UninstallControllers phase prepared, %d controllers will be uninstalled", len(p.hosts))
	return nil
}

// ShouldRun is true when there are controllers that needs to be upgraded
func (p *UninstallControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// CleanUp cleans up the environment override files on hosts
func (p *UninstallControllers) CleanUp() {
	for _, h := range p.hosts {
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
	}
}

// Run the phase
func (p *UninstallControllers) Run() error {
	for _, h := range p.hosts {
		log.Debugf("%s: draining node", h)
		if !p.NoDrain && h.Role != "controller" {
			if err := p.leader.DrainNode(&cluster.Host{
				Metadata: cluster.HostMetadata{
					Hostname: h.Metadata.Hostname,
				},
			}); err != nil {
				log.Warnf("%s: failed to drain node: %s", h, err.Error())
			}
		}
		log.Debugf("%s: draining node completed", h)

		log.Debugf("%s: deleting node...", h)
		if h.Role != "controller" {
			if err := p.leader.DeleteNode(&cluster.Host{
				Metadata: cluster.HostMetadata{
					Hostname: h.Metadata.Hostname,
				},
			}); err != nil {
				log.Warnf("%s: failed to delete node: %s", h, err.Error())
			}
		}
		log.Debugf("%s: deleting node", h)

		if h.Configurer.ServiceIsRunning(h, h.K0sServiceName()) {
			log.Debugf("%s: stopping k0s...", h)
			if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to stop k0s: %s", h, err.Error())
			}
			log.Debugf("%s: waiting for k0s to stop", h)
			if err := h.WaitK0sServiceStopped(); err != nil {
				log.Warnf("%s: failed to wait for k0s to stop: %s", h, err.Error())
			}
			log.Debugf("%s: stopping k0s completed", h)
		}

		log.Debugf("%s: leaving etcd...", h)
		if err := p.leader.LeaveEtcd(h); err != nil {
			log.Warnf("%s: failed to leave etcd: %s", h, err.Error())
		}
		log.Debugf("%s: leaving etcd completed", h)

		log.Debugf("%s: resetting k0s...", h)
		out, err := h.ExecOutput(h.Configurer.K0sCmdf("reset"), exec.Sudo(h))
		c, _ := semver.NewConstraint("<= 1.22.3+k0s.0")
		running, _ := semver.NewVersion(h.Metadata.K0sBinaryVersion)
		if err != nil {
			log.Warnf("%s: k0s reported failure: %v", h, err)
			if c.Check(running) && !strings.Contains(out, "k0s cleanup operations done") {
				log.Warnf("%s: k0s reset failed, trying k0s cleanup", h)
			}
		}
		log.Debugf("%s: resetting k0s completed", h)

		log.Debugf("%s: removing config...", h)
		if dErr := h.Configurer.DeleteFile(h, h.Configurer.K0sConfigPath()); dErr != nil {
			log.Warnf("%s: failed to remove existing configuration %s: %s", h, h.Configurer.K0sConfigPath(), dErr)
		}
		log.Debugf("%s: removing config completed", h)

		log.Infof("%s: uninstalled", h)
	}
	return nil
}
