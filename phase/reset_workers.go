package phase

import (
	"bytes"
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
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
	return p.parallelDo(ctx, p.hosts, func(_ context.Context, h *cluster.Host) error {
		if t := p.Config.Spec.Options.EvictTaint; t.Enabled {
			log.Debugf("%s: add taint: %s", h, t.String())
			if err := p.leader.AddTaint(h, t.String()); err != nil {
				return fmt.Errorf("add taint: %w", err)
			}
		}
		if !p.NoDrain {
			log.Debugf("%s: draining node", h)
			if err := p.leader.DrainNode(
				&cluster.Host{
					Metadata: cluster.HostMetadata{
						Hostname: h.Metadata.Hostname,
					},
				},
				p.Config.Spec.Options.Drain,
			); err != nil {
				log.Warnf("%s: failed to drain node: %s", h, err.Error())
			}
		}
		log.Debugf("%s: draining node completed", h)

		log.Debugf("%s: deleting node...", h)
		if !p.NoDelete {
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
			if err := retry.WithDefaultTimeout(ctx, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
				log.Warnf("%s: failed to wait for k0s to stop: %s", h, err.Error())
			}
			log.Debugf("%s: stopping k0s completed", h)
		}

		log.Debugf("%s: resetting k0s...", h)
		var stdoutbuf, stderrbuf bytes.Buffer
		cmd, err := h.ExecStreams(h.K0sResetCommand(), nil, &stdoutbuf, &stderrbuf, exec.Sudo(h))
		if err != nil {
			return fmt.Errorf("failed to run k0s reset: %w", err)
		}
		if err := cmd.Wait(); err != nil {
			log.Warnf("%s: k0s reset reported failure: %s %s", h, stderrbuf.String(), stdoutbuf.String())
		}
		log.Debugf("%s: resetting k0s completed", h)

		log.Debugf("%s: removing config...", h)
		if dErr := h.Configurer.DeleteFile(h, h.Configurer.K0sConfigPath()); dErr != nil {
			log.Warnf("%s: failed to remove existing configuration %s: %s", h, h.Configurer.K0sConfigPath(), dErr)
		}
		log.Debugf("%s: removing config completed", h)

		log.Debugf("%s: removing k0s binary...", h)
		if dErr := h.Configurer.DeleteFile(h, h.Configurer.K0sBinaryPath()); dErr != nil {
			log.Warnf("%s: failed to remove existing binary %s: %s", h, h.Configurer.K0sConfigPath(), dErr)
		}
		log.Debugf("%s: removing binary completed", h)

		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}

		log.Infof("%s: reset", h)
		return nil
	})
}
