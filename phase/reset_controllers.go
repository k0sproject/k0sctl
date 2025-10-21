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
		if t := p.Config.Spec.Options.EvictTaint; t.Enabled && t.ControllerWorkers && h.Role != "controller" {
			log.Debugf("%s: add taint: %s", h, t.String())
			if err := p.leader.AddTaint(h, t.String()); err != nil {
				return fmt.Errorf("add taint: %w", err)
			}
		}
		if !p.NoDrain && h.Role != "controller" {
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

		if !p.NoDelete && h.Role != "controller" {
			log.Debugf("%s: deleting node...", h)
			if err := p.leader.DeleteNode(&cluster.Host{
				Metadata: cluster.HostMetadata{
					Hostname: h.Metadata.Hostname,
				},
			}); err != nil {
				log.Warnf("%s: failed to delete node: %s", h, err.Error())
			}
		}

		if h.Configurer.ServiceIsRunning(h, h.K0sServiceName()) {
			log.Debugf("%s: stopping k0s...", h)
			if err := h.Configurer.StopService(h, h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to stop k0s: %s", h, err.Error())
			}
			log.Debugf("%s: waiting for k0s to stop", h)
			if err := retry.WithDefaultTimeout(ctx, node.ServiceStoppedFunc(h, h.K0sServiceName())); err != nil {
				log.Warnf("%s: failed to wait for k0s to stop: %v", h, err)
			}
			log.Debugf("%s: stopping k0s completed", h)
		}

		if !p.NoLeave {
			log.Debugf("%s: leaving etcd...", h)

			if err := h.Exec(h.Configurer.K0sCmdf("etcd leave --peer-address %s --datadir %s", h.PrivateAddress, h.K0sDataDir()), exec.Sudo(h)); err != nil {
				log.Warnf("%s: failed to leave etcd: %s", h, err.Error())
			}
			log.Debugf("%s: leaving etcd completed", h)
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
	}
	return nil
}
