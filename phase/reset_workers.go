package phase

import (
	"bytes"
	"context"
	"fmt"

	log "github.com/k0sproject/k0sctl/internal/log"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
)

// ResetControllers phase removes workers marked for reset from the kubernetes cluster
// and resets k0s on the host
type ResetWorkers struct {
	GenericPhase

	NoDrain  bool
	NoDelete bool

	hosts  cluster.Hosts
	leader *cluster.Host
}

// Title for the phase
func (p *ResetWorkers) Title() string {
	return "Reset workers"
}

// Prepare the phase
func (p *ResetWorkers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	workers := p.Config.Spec.Hosts.Workers()
	log.Debugf("%d workers in total", len(workers))
	p.hosts = workers.Filter(func(h *cluster.Host) bool {
		return h.Reset
	})
	log.Debugf("ResetWorkers phase prepared, %d workers will be reset", len(p.hosts))
	return nil
}

// Before runs "before reset" hooks
func (p *ResetWorkers) Before() error {
	return p.runHooks(context.Background(), "reset", "before", p.hosts...)
}

// After runs "after reset" hooks
func (p *ResetWorkers) After() error {
	return p.runHooks(context.Background(), "reset", "after", p.hosts...)
}

// ShouldRun is true when there are workers that needs to be reset
func (p *ResetWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// DryRun reports the nodes will be reset
func (p *ResetWorkers) DryRun() error {
	for _, h := range p.hosts {
		p.DryMsg(h, "node would be reset")
	}
	return nil
}

// Run the phase
func (p *ResetWorkers) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.hosts, func(ctx context.Context, h *cluster.Host) error {
		if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
			h.Log().Debugf("add taint: %s", t.String())
			if err := p.leader.AddTaint(h, t.String()); err != nil {
				return fmt.Errorf("add taint: %w", err)
			}
		}
		if !p.NoDrain {
			h.Log().Debugf("draining node")
			if err := p.leader.DrainNode(
				&cluster.Host{
					Metadata: cluster.HostMetadata{
						Hostname: h.KubernetesNodeName(),
					},
				},
				p.Config.Spec.Options.Drain,
			); err != nil {
				h.Log().Warnf("failed to drain node: %s", err.Error())
			}
		}
		h.Log().Debugf("draining node completed")

		h.Log().Debugf("deleting node...")
		if !p.NoDelete {
			if err := p.leader.DeleteNode(&cluster.Host{
				Metadata: cluster.HostMetadata{
					Hostname: h.KubernetesNodeName(),
				},
			}); err != nil {
				h.Log().Warnf("failed to delete node: %s", err.Error())
			}
		}
		h.Log().Debugf("deleting node")

		if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
			h.Log().Warnf("failed to get service %s: %v", h.K0sServiceName(), err)
		} else if svc.IsRunning(ctx) {
			h.Log().Debugf("stopping k0s...")
			if err := svc.Stop(ctx); err != nil {
				h.Log().Warnf("failed to stop k0s: %s", err.Error())
			}
			h.Log().Debugf("waiting for k0s to stop")
			if err := retry.WithDefaultTimeout(ctx, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
				h.Log().Warnf("failed to wait for k0s to stop: %s", err.Error())
			}
			h.Log().Debugf("stopping k0s completed")
		}

		h.Log().Debugf("resetting k0s...")
		var stdoutbuf, stderrbuf bytes.Buffer
		proc := h.Sudo().Proc(h.K0sResetCommand())
		proc.Stdout = &stdoutbuf
		proc.Stderr = &stderrbuf
		waiter, err := proc.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to run k0s reset: %w", err)
		}
		if err := waiter.Wait(); err != nil {
			h.Log().Warnf("k0s reset reported failure: %s %s", stderrbuf.String(), stdoutbuf.String())
		}
		h.Log().Debugf("resetting k0s completed")

		h.Log().Debugf("removing config...")
		if dErr := h.Sudo().FS().Remove(h.Configurer.K0sConfigPath()); dErr != nil {
			h.Log().Warnf("failed to remove existing configuration %s: %s", h.Configurer.K0sConfigPath(), dErr)
		}
		h.Log().Debugf("removing config completed")

		h.Log().Debugf("removing k0s binary...")
		if dErr := h.Sudo().FS().Remove(h.Configurer.K0sBinaryPath()); dErr != nil {
			h.Log().Warnf("failed to remove existing binary %s: %s", h.Configurer.K0sConfigPath(), dErr)
		}
		h.Log().Debugf("removing binary completed")

		if len(h.Environment) > 0 {
			if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
				h.Log().Warnf("failed to get service %s: %v", h.K0sServiceName(), err)
			} else if err := svc.SetEnvironment(ctx, map[string]string{}); err != nil {
				h.Log().Warnf("failed to clean up service environment: %s", err.Error())
			}
		}

		h.Log().Infof("reset")
		return nil
	})
}
