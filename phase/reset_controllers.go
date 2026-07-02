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

// ResetControllers phase removes controllers marked for reset from the kubernetes and etcd clusters
// and resets k0s on the host
type ResetControllers struct {
	GenericPhase

	NoDrain  bool
	NoDelete bool
	NoLeave  bool

	hosts  cluster.Hosts
	leader *cluster.Host
}

// Title for the phase
func (p *ResetControllers) Title() string {
	return "Reset controllers"
}

// Before runs "before reset" hooks
func (p *ResetControllers) Before() error {
	return p.runHooks(context.Background(), "reset", "before", p.hosts...)
}

// After runs "after reset" hooks
func (p *ResetControllers) After() error {
	return p.runHooks(context.Background(), "reset", "after", p.hosts...)
}

// Prepare the phase
func (p *ResetControllers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	controllers := p.Config.Spec.Hosts.Controllers()
	log.Debugf("%d controllers in total", len(controllers))
	p.hosts = controllers.Filter(func(h *cluster.Host) bool {
		return h.Reset
	})
	log.Debugf("ResetControllers phase prepared, %d controllers will be reset", len(p.hosts))
	return nil
}

// ShouldRun is true when there are controllers that needs to be reset
func (p *ResetControllers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// DryRun reports nodes that would get reset
func (p *ResetControllers) DryRun() error {
	for _, h := range p.hosts {
		p.DryMsg(h, "reset node")
	}
	return nil
}

// Run the phase
func (p *ResetControllers) Run(ctx context.Context) error {
	for _, h := range p.hosts {
		ctx := log.IntoContext(ctx, h.Log())
		if t := p.Config.Spec.Options.EvictTaint; t.Enabled && t.ControllerWorkers && h.Role != "controller" {
			h.Log().Debugf("add taint: %s", t.String())
			if err := p.leader.AddTaint(h, t.String()); err != nil {
				return fmt.Errorf("add taint: %w", err)
			}
		}
		if !p.NoDrain && h.Role != "controller" {
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

		if !p.NoDelete && h.Role != "controller" {
			h.Log().Debugf("deleting node...")
			if err := p.leader.DeleteNode(&cluster.Host{
				Metadata: cluster.HostMetadata{
					Hostname: h.KubernetesNodeName(),
				},
			}); err != nil {
				h.Log().Warnf("failed to delete node: %s", err.Error())
			}
		}

		if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
			h.Log().Warnf("failed to get service %s: %v", h.K0sServiceName(), err)
		} else if svc.IsRunning(ctx) {
			h.Log().Debugf("stopping k0s...")
			if err := svc.Stop(ctx); err != nil {
				h.Log().Warnf("failed to stop k0s: %s", err.Error())
			}
			h.Log().Debugf("waiting for k0s to stop")
			if err := retry.WithDefaultTimeout(ctx, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
				h.Log().Warnf("failed to wait for k0s to stop: %v", err)
			}
			h.Log().Debugf("stopping k0s completed")
		}

		if !p.NoLeave {
			h.Log().Debugf("leaving etcd...")

			if err := h.Sudo().Exec(h.Configurer.K0sCmdf("etcd leave --peer-address %s --datadir %s", h.PrivateAddress, h.K0sDataDir())); err != nil {
				h.Log().Warnf("failed to leave etcd: %s", err.Error())
			}
			h.Log().Debugf("leaving etcd completed")
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
	}
	return nil
}
