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

	var workers cluster.Hosts = p.Config.Spec.Hosts.Workers()
	log.Debugf("%d workers in total", len(workers))
	p.hosts = workers.Filter(func(h *cluster.Host) bool {
		return h.Reset
	})
	log.Debugf("ResetWorkers phase prepared, %d workers will be reset", len(p.hosts))
	return nil
}

// Before runs "before reset" hooks
func (p *ResetWorkers) Before() error {
	return p.hosts.ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		if !p.IsWet() && h.HasHooks("reset", "before") {
			p.DryMsg(h, "run before reset hooks")
			return nil
		}

		if err := h.RunHooks("reset", "before"); err != nil {
			return fmt.Errorf("failed to run before reset hooks: %w", err)
		}

		return nil
	})
}

// After runs "after reset" hooks
func (p *ResetWorkers) After() error {
	return p.hosts.ParallelEach(context.Background(), func(_ context.Context, h *cluster.Host) error {
		if !p.IsWet() && h.HasHooks("reset", "after") {
			p.DryMsg(h, "run after reset hooks")
			return nil
		}

		if err := h.RunHooks("reset", "after"); err != nil {
			return fmt.Errorf("failed to run after reset hooks: %w", err)
		}

		return nil
	})
}

// ShouldRun is true when there are workers that needs to be reset
func (p *ResetWorkers) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *ResetWorkers) Run(ctx context.Context) error {
	return p.parallelDo(ctx, p.hosts, func(_ context.Context, h *cluster.Host) error {
		log.Debugf("%s: draining node", h)
		if !p.NoDrain {
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
			if err := retry.AdaptiveTimeout(ctx, retry.DefaultTimeout, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
				log.Warnf("%s: failed to wait for k0s to stop: %s", h, err.Error())
			}
			log.Debugf("%s: stopping k0s completed", h)
		}

		log.Debugf("%s: resetting k0s...", h)
		var stdoutbuf, stderrbuf bytes.Buffer
		cmd, err := h.ExecStreams(h.Configurer.K0sCmdf("reset --data-dir=%s", h.K0sDataDir()), nil, &stdoutbuf, &stderrbuf, exec.Sudo(h))
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

		if len(h.Environment) > 0 {
			if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}

		log.Infof("%s: reset", h)
		return nil
	})
}
