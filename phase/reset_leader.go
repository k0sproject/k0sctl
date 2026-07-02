package phase

import (
	"context"
	"fmt"

	log "github.com/k0sproject/k0sctl/internal/log"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
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
	ctx = log.IntoContext(ctx, p.leader.Log())
	if t := p.Config.Spec.Options.EvictTaint; t.Enabled && t.ControllerWorkers && p.leader.Role != "controller" {
		p.leader.Log().Debugf("add taint %s", t.String())
		if err := p.leader.AddTaint(p.leader, t.String()); err != nil {
			return fmt.Errorf("add taint: %w", err)
		}
	}

	if leaderSvc, err := p.leader.Sudo().Service(p.leader.K0sServiceName()); err != nil {
		p.leader.Log().Warnf("failed to get service %s: %v", p.leader.K0sServiceName(), err)
	} else if leaderSvc.IsRunning(ctx) {
		p.leader.Log().Debugf("stopping k0s...")
		if err := leaderSvc.Stop(ctx); err != nil {
			p.leader.Log().Warnf("failed to stop k0s: %s", err.Error())
		}
		p.leader.Log().Debugf("waiting for k0s to stop")
		if err := retry.WithDefaultTimeout(ctx, node.ServiceStoppedFunc(p.leader, p.leader.K0sServiceName())); err != nil {
			p.leader.Log().Warnf("k0s service stop: %s", err.Error())
		}
		p.leader.Log().Debugf("stopping k0s completed")
	}

	p.leader.Log().Debugf("resetting k0s...")
	out, err := p.leader.Sudo().ExecOutput(p.leader.K0sResetCommand())
	if err != nil {
		p.leader.Log().Debugf("k0s reset failed: %s", out)
		p.leader.Log().Warnf("k0s reported failure: %v", err)
	}
	p.leader.Log().Debugf("resetting k0s completed")

	p.leader.Log().Debugf("removing config...")
	if dErr := p.leader.Sudo().FS().Remove(p.leader.Configurer.K0sConfigPath()); dErr != nil {
		p.leader.Log().Warnf("failed to remove existing configuration %s: %s", p.leader.Configurer.K0sConfigPath(), dErr)
	}
	p.leader.Log().Debugf("removing config completed")

	p.leader.Log().Debugf("removing k0s binary...")
	if dErr := p.leader.Sudo().FS().Remove(p.leader.Configurer.K0sBinaryPath()); dErr != nil {
		p.leader.Log().Warnf("failed to remove existing binary %s: %s", p.leader.Configurer.K0sConfigPath(), dErr)
	}
	p.leader.Log().Debugf("removing binary completed")

	if len(p.leader.Environment) > 0 {
		if svc, err := p.leader.Sudo().Service(p.leader.K0sServiceName()); err != nil {
			p.leader.Log().Warnf("failed to get service %s: %v", p.leader.K0sServiceName(), err)
		} else if err := svc.SetEnvironment(ctx, map[string]string{}); err != nil {
			p.leader.Log().Warnf("failed to clean up service environment: %s", err.Error())
		}
	}

	p.leader.Log().Infof("reset")

	return nil
}
