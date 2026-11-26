package phase

import (
	"bytes"
	"context"
	"fmt"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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
						Hostname: h.KubernetesNodeName(),
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
					Hostname: h.KubernetesNodeName(),
				},
			}); err != nil {
				log.Warnf("%s: failed to delete node: %s", h, err.Error())
			}
		}

		if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
			log.Warnf("%s: failed to get service %s: %v", h, h.K0sServiceName(), err)
		} else if svc.IsRunning(ctx) {
			log.Debugf("%s: stopping k0s...", h)
			if err := svc.Stop(ctx); err != nil {
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

			if err := h.Sudo().Exec(h.Configurer.K0sCmdf("etcd leave --peer-address %s --datadir %s", installedEtcdPeerAddress(h), h.K0sDataDir())); err != nil {
				log.Warnf("%s: failed to leave etcd: %s", h, err.Error())
			}
			log.Debugf("%s: leaving etcd completed", h)
		}

		log.Debugf("%s: resetting k0s...", h)
		var stdoutbuf, stderrbuf bytes.Buffer
		proc := h.Sudo().Proc(h.K0sResetCommand())
		proc.Stdout = &stdoutbuf
		proc.Stderr = &stderrbuf
		waiter, err := proc.Start(ctx)
		if err != nil {
			return fmt.Errorf("failed to run k0s reset: %w", err)
		}
		if err := waiter.Wait(); err != nil {
			log.Warnf("%s: k0s reset reported failure: %s %s", h, stderrbuf.String(), stdoutbuf.String())
		}
		log.Debugf("%s: resetting k0s completed", h)

		log.Debugf("%s: removing config...", h)
		if dErr := h.Sudo().FS().Remove(h.Configurer.K0sConfigPath()); dErr != nil {
			log.Warnf("%s: failed to remove existing configuration %s: %s", h, h.Configurer.K0sConfigPath(), dErr)
		}
		log.Debugf("%s: removing config completed", h)

		log.Debugf("%s: removing k0s binary...", h)
		if dErr := h.Sudo().FS().Remove(h.Configurer.K0sBinaryPath()); dErr != nil {
			log.Warnf("%s: failed to remove existing binary %s: %s", h, h.Configurer.K0sConfigPath(), dErr)
		}
		log.Debugf("%s: removing binary completed", h)

		if len(h.Environment) > 0 {
			if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
				log.Warnf("%s: failed to get service %s: %v", h, h.K0sServiceName(), err)
			} else if err := svc.SetEnvironment(ctx, map[string]string{}); err != nil {
				log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
			}
		}

		log.Infof("%s: reset", h)
	}
	return nil
}

// installedEtcdPeerAddress returns the etcd peer address from the k0s config
// installed on the host. A reset host is not reconfigured, so a changed
// peerAddress in its host k0s config has not been applied yet. Without an
// installed peerAddress, it falls back to the private address, or the
// connection address when there is no private address.
func installedEtcdPeerAddress(h *cluster.Host) string {
	if h.Metadata.K0sExistingConfig != "" {
		cfg := make(dig.Mapping)
		if err := yaml.Unmarshal([]byte(h.Metadata.K0sExistingConfig), &cfg); err != nil {
			log.Debugf("%s: failed to parse existing k0s config for the etcd peer address: %v", h, err)
		} else if addr := cfg.DigString("spec", "storage", "etcd", "peerAddress"); addr != "" {
			return addr
		}
	}
	if h.PrivateAddress != "" {
		return h.PrivateAddress
	}
	return h.Address()
}
