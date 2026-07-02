package phase

import (
	"context"
	"fmt"
	"strings"

	log "github.com/k0sproject/k0sctl/internal/log"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
)

// InitializeK0s sets up the "initial" k0s controller
type InitializeK0s struct {
	GenericPhase
	leader *cluster.Host
}

// Title for the phase
func (p *InitializeK0s) Title() string {
	return "Initialize the k0s cluster"
}

// Prepare the phase
func (p *InitializeK0s) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	leader := p.Config.Spec.K0sLeader()
	if leader.Metadata.K0sRunningVersion == nil {
		p.leader = leader
	}
	return nil
}

// Before runs "before install" hooks for the leader controller
func (p *InitializeK0s) Before() error {
	if p.leader == nil || p.leader.Reset {
		return nil
	}
	return p.runHooks(context.Background(), "install", "before", p.leader)
}

// After runs "after install" hooks for the leader controller
func (p *InitializeK0s) After() error {
	if p.leader == nil || p.leader.Reset {
		return nil
	}
	return p.runHooks(context.Background(), "install", "after", p.leader)
}

// ShouldRun is true when there is a leader host
func (p *InitializeK0s) ShouldRun() bool {
	return p.leader != nil && !p.leader.Reset
}

// CleanUp cleans up the environment override file
func (p *InitializeK0s) CleanUp() {
	h := p.leader

	h.Log().Infof("cleaning up")
	if len(h.Environment) > 0 {
		if svc, err := h.Sudo().Service(h.K0sServiceName()); err != nil {
			h.Log().Warnf("failed to get service %s: %v", h.K0sServiceName(), err)
		} else if err := svc.SetEnvironment(context.Background(), map[string]string{}); err != nil {
			h.Log().Warnf("failed to clean up service environment: %s", err.Error())
		}
	}
	if h.Metadata.K0sInstalled {
		if err := h.Sudo().Exec(h.K0sResetCommand()); err != nil {
			h.Log().Warnf("k0s reset failed")
		}
	}
}

// Run the phase
func (p *InitializeK0s) Run(ctx context.Context) error {
	h := p.leader
	ctx = log.IntoContext(ctx, h.Log())
	h.Metadata.IsK0sLeader = true

	if p.Config.Spec.K0s.DynamicConfig || (h.InstallFlags.Include("--enable-dynamic-config") && h.InstallFlags.GetValue("--enable-dynamic-config") != "false") {
		p.Config.Spec.K0s.DynamicConfig = true
		h.InstallFlags.AddOrReplace("--enable-dynamic-config")
	}

	if Force {
		h.Log().Warnf("--force given, using k0s install with --force")
		h.InstallFlags.AddOrReplace("--force=true")
	}

	h.Log().Infof("installing k0s controller")
	cmd, err := h.K0sInstallCommand()
	if err != nil {
		return err
	}

	err = p.Wet(p.leader, fmt.Sprintf("install first k0s controller using `%s`", strings.ReplaceAll(cmd, p.leader.K0sInstallLocation(), "k0s")), func() error {
		return h.Sudo().Exec(cmd)
	}, func() error {
		p.leader.Metadata.DryRunFakeLeader = true
		return nil
	})
	if err != nil {
		return err
	}

	h.Metadata.K0sInstalled = true

	if len(h.Environment) > 0 {
		err = p.Wet(h, "configure k0s service environment variables", func() error {
			h.Log().Infof("updating service environment")
			svc, err := h.Sudo().Service(h.K0sServiceName())
			if err != nil {
				return fmt.Errorf("get service %s: %w", h.K0sServiceName(), err)
			}
			return svc.SetEnvironment(ctx, h.Environment)
		}, func() error {
			for k, v := range h.Environment {
				p.DryMsgf(h, "%s=<%d characters>", k, len(v))
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	err = p.Wet(h, "start k0s service", func() error {
		svc, err := h.Sudo().Service(h.K0sServiceName())
		if err != nil {
			return fmt.Errorf("get service %s: %w", h.K0sServiceName(), err)
		}
		if err := svc.Start(ctx); err != nil {
			return err
		}

		h.Log().Infof("waiting for the k0s service to start")
		if err := retry.WithDefaultTimeout(ctx, node.ServiceRunningFunc(h, h.K0sServiceName())); err != nil {
			return err
		}

		h.Log().Infof("wait for kubernetes to reach ready state")
		err = retry.WithDefaultTimeout(ctx, func(_ context.Context) error {
			out, err := h.Sudo().ExecOutput(h.Configurer.KubectlCmdf(h, h.K0sDataDir(), "get --raw='/readyz'"))
			if out != "ok" {
				return fmt.Errorf("kubernetes api /readyz responded with %q", out)
			}
			return err
		})
		if err != nil {
			return fmt.Errorf("kubernetes not ready: %w", err)
		}

		h.Metadata.Ready = true

		return nil
	})
	if err != nil {
		return err
	}

	if p.IsWet() && p.Config.Spec.K0s.DynamicConfig {
		if err := retry.WithDefaultTimeout(ctx, node.K0sDynamicConfigReadyFunc(h)); err != nil {
			return fmt.Errorf("dynamic config reconciliation failed: %w", err)
		}
	}

	h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
	h.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version
	h.Metadata.Ready = true

	if p.IsWet() {
		if id, err := p.Config.Spec.K0s.GetClusterID(h); err == nil {
			p.Config.Spec.K0s.Metadata.ClusterID = id
		}
	}

	return nil
}
