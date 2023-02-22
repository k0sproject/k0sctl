package phase

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

// ResetLeader phase removes the leader from the cluster and thus destroys the cluster
type ResetLeader struct {
	GenericPhase
	leader *cluster.Host
}

// Title for the phase
func (p *ResetLeader) Title() string {
	return "Reset leader"
}

// Prepare the phase
func (p *ResetLeader) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()
	return nil
}

// CleanUp cleans up the environment override files on hosts
func (p *ResetLeader) CleanUp() {
	if len(p.leader.Environment) > 0 {
		if err := p.leader.Configurer.CleanupServiceEnvironment(p.leader, p.leader.K0sServiceName()); err != nil {
			log.Warnf("%s: failed to clean up service environment: %s", p.leader, err.Error())
		}
	}
}

// Run the phase
func (p *ResetLeader) Run() error {
	if p.leader.Configurer.ServiceIsRunning(p.leader, p.leader.K0sServiceName()) {
		log.Debugf("%s: stopping k0s...", p.leader)
		if err := p.leader.Configurer.StopService(p.leader, p.leader.K0sServiceName()); err != nil {
			log.Warnf("%s: failed to stop k0s: %s", p.leader, err.Error())
		}
		log.Debugf("%s: waiting for k0s to stop", p.leader)
		if err := p.leader.WaitK0sServiceStopped(); err != nil {
			log.Warnf("%s: failed to wait for k0s to stop: %s", p.leader, err.Error())
		}
		log.Debugf("%s: stopping k0s completed", p.leader)
	}

	log.Debugf("%s: resetting k0s...", p.leader)
	out, err := p.leader.ExecOutput(p.leader.Configurer.K0sCmdf("reset --data-dir=%s", p.leader.DataDir), exec.Sudo(p.leader))
	if err != nil {
		log.Debugf("%s: k0s reset failed: %s", p.leader, out)
		log.Warnf("%s: k0s reported failure: %v", p.leader, err)
	}
	log.Debugf("%s: resetting k0s completed", p.leader)

	log.Debugf("%s: removing config...", p.leader)
	if dErr := p.leader.Configurer.DeleteFile(p.leader, p.leader.Configurer.K0sConfigPath()); dErr != nil {
		log.Warnf("%s: failed to remove existing configuration %s: %s", p.leader, p.leader.Configurer.K0sConfigPath(), dErr)
	}
	log.Debugf("%s: removing config completed", p.leader)

	log.Infof("%s: reset", p.leader)

	return nil
}
