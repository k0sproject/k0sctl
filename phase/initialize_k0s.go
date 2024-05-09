package phase

import (
	"context"
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
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

// ShouldRun is true when there is a leader host
func (p *InitializeK0s) ShouldRun() bool {
	return p.leader != nil && !p.leader.Reset
}

// CleanUp cleans up the environment override file
func (p *InitializeK0s) CleanUp() {
	h := p.leader

	log.Infof("%s: cleaning up", h)
	if len(h.Environment) > 0 {
		if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
			log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
		}
	}
	if h.Metadata.K0sInstalled {
		if err := h.Exec(h.Configurer.K0sCmdf("reset --data-dir=%s", h.K0sDataDir()), exec.Sudo(h)); err != nil {
			log.Warnf("%s: k0s reset failed", h)
		}
	}
}

// Run the phase
func (p *InitializeK0s) Run() error {
	h := p.leader
	h.Metadata.IsK0sLeader = true

	if p.Config.Spec.K0s.DynamicConfig || (h.InstallFlags.Include("--enable-dynamic-config") && h.InstallFlags.GetValue("--enable-dynamic-config") != "false") {
		p.Config.Spec.K0s.DynamicConfig = true
		h.InstallFlags.AddOrReplace("--enable-dynamic-config")
	}

	if Force {
		log.Warnf("%s: --force given, using k0s install with --force", h)
		h.InstallFlags.AddOrReplace("--force=true")
	}

	log.Infof("%s: installing k0s controller", h)
	cmd, err := h.K0sInstallCommand()
	if err != nil {
		return err
	}

	err = p.Wet(p.leader, fmt.Sprintf("install first k0s controller using `%s`", strings.ReplaceAll(cmd, p.leader.Configurer.K0sBinaryPath(), "k0s")), func() error {
		return h.Exec(cmd, exec.Sudo(h))
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
			log.Infof("%s: updating service environment", h)
			return h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment)
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
		if err := h.Configurer.StartService(h, h.K0sServiceName()); err != nil {
			return err
		}

		log.Infof("%s: waiting for the k0s service to start", h)
		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.ServiceRunningFunc(h, h.K0sServiceName())); err != nil {
			return err
		}

		port := 6443
		if p, ok := p.Config.Spec.K0s.Config.Dig("spec", "api", "port").(int); ok {
			port = p
		}
		log.Infof("%s: waiting for kubernetes api to respond", h)
		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.KubeAPIReadyFunc(h, port)); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	if p.IsWet() && p.Config.Spec.K0s.DynamicConfig {
		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.K0sDynamicConfigReadyFunc(h)); err != nil {
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
