package phase

import (
	"time"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// InstallControllers installs k0s controllers and joins them to the cluster
type InstallControllers struct {
	GenericPhase
	hosts  cluster.Hosts
	leader *cluster.Host
}

// Title for the phase
func (p *InstallControllers) Title() string {
	return "Install controllers"
}

// Prepare the phase
func (p *InstallControllers) Prepare(config *config.Cluster) error {
	p.Config = config
	var controllers cluster.Hosts = p.Config.Spec.Hosts.Controllers()
	p.leader = p.Config.Spec.K0sLeader()
	p.hosts = controllers.Filter(func(h *cluster.Host) bool {
		return h != p.leader && h.Metadata.K0sRunningVersion == ""
	})

	return nil
}

// ShouldRun is true when there are controllers
func (p *InstallControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// CleanUp cleans up the environment override files on hosts
func (p *InstallControllers) CleanUp() {
	for _, h := range p.hosts {
		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}
	}
}

// Run the phase
func (p *InstallControllers) Run() error {
	for _, h := range p.hosts {
		log.Infof("%s: generating token", p.leader)
		token, err := p.Config.Spec.K0s.GenerateToken(
			p.leader,
			"controller",
			time.Duration(10)*time.Minute,
		)
		if err != nil {
			return err
		}
		tokenID, err := cluster.TokenID(token)
		if err != nil {
			return err
		}
		log.Debugf("%s: join token ID: %s", p.leader, tokenID)
		defer func() {
			if err := p.leader.Exec(p.leader.Configurer.K0sCmdf("token invalidate %s", tokenID), exec.Sudo(p.leader), exec.RedactString(token)); err != nil {
				log.Warnf("%s: failed to invalidate the controller join token", p.leader)
			}
		}()

		log.Infof("%s: writing join token", h)
		if err := h.Configurer.WriteFile(h, h.K0sJoinTokenPath(), token, "0640"); err != nil {
			return err
		}

		defer func() {
			if err := h.Configurer.DeleteFile(h, h.K0sJoinTokenPath()); err != nil {
				log.Warnf("%s: failed to clean up the join token file at %s", h, h.K0sJoinTokenPath())
			}
		}()

		log.Infof("%s: installing k0s controller", h)
		if err := h.Exec(h.K0sInstallCommand()); err != nil {
			return err
		}

		if len(h.Environment) > 0 {
			log.Infof("%s: updating service environment", h)
			if err := h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment); err != nil {
				return err
			}
		}

		log.Infof("%s: starting service", h)
		if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
			return err
		}

		log.Infof("%s: waiting for the k0s service to start", h)
		if err := h.WaitK0sServiceRunning(); err != nil {
			return err
		}

		if err := p.waitJoined(h); err != nil {
			return err
		}
	}

	return nil
}

func (p *InstallControllers) waitJoined(h *cluster.Host) error {
	port := 6443
	if p, ok := p.Config.Spec.K0s.Config.Dig("spec", "api", "port").(int); ok {
		port = p
	}

	log.Infof("%s: waiting for kubernetes api to respond", h)
	return h.WaitKubeAPIReady(port)
}
