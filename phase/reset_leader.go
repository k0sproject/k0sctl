package phase

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
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

// Before runs "before reset" hooks
func (p *ResetLeader) Before() error {
	return p.runHooks(context.Background(), "reset", "before", p.leader)
}

// After runs "after backup" hooks
func (p *ResetLeader) After() error {
	return p.runHooks(context.Background(), "reset", "after", p.leader)
}

// Prepare the phase
func (p *ResetLeader) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()
	return nil
}

// DryRun reports that the host will be reset
func (p *ResetLeader) DryRun() error {
	p.DryMsg(p.leader, "reset node")
	return nil
}

// Run the phase
func (p *ResetLeader) Run(ctx context.Context) error {
	if t := p.Config.Spec.Options.EvictTaint; t.Enabled && t.ControllerWorkers && p.leader.Role != "controller" {
		log.Debugf("%s: add taint %s", p.leader, t.String())
		if err := p.leader.AddTaint(p.leader, t.String()); err != nil {
			return fmt.Errorf("add taint: %w", err)
		}
	}

	if p.leader.Configurer.ServiceIsRunning(p.leader, p.leader.K0sServiceName()) {
		log.Debugf("%s: stopping k0s...", p.leader)
		if err := p.leader.Configurer.StopService(p.leader, p.leader.K0sServiceName()); err != nil {
			log.Warnf("%s: failed to stop k0s: %s", p.leader, err.Error())
		}
		log.Debugf("%s: waiting for k0s to stop", p.leader)
		if err := retry.Timeout(ctx, p.Config.Spec.Options.Timeout.ServiceStop, node.ServiceStoppedFunc(p.leader, p.leader.K0sServiceName())); err != nil {
			log.Warnf("%s: k0s service stop: %s", p.leader, err.Error())
		}
		log.Debugf("%s: stopping k0s completed", p.leader)
	}

	log.Debugf("%s: resetting k0s...", p.leader)
	out, err := p.leader.ExecOutput(p.leader.K0sResetCommand(), exec.Sudo(p.leader))
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

	log.Debugf("%s: removing k0s binary...", p.leader)
	if dErr := p.leader.Configurer.DeleteFile(p.leader, p.leader.Configurer.K0sBinaryPath()); dErr != nil {
		log.Warnf("%s: failed to remove existing binary %s: %s", p.leader, p.leader.Configurer.K0sConfigPath(), dErr)
	}
	log.Debugf("%s: removing binary completed", p.leader)

	if len(p.leader.Environment) > 0 {
		if err := p.leader.Configurer.CleanupServiceEnvironment(p.leader, p.leader.K0sServiceName()); err != nil {
			log.Warnf("%s: failed to clean up service environment: %s", p.leader, err.Error())
		}
	}

	log.Infof("%s: reset", p.leader)

	return nil
}
