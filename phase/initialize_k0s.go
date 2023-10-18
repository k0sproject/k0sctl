package phase

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/node"
	"github.com/k0sproject/k0sctl/pkg/retry"
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
	return p.leader != nil
}

// CleanUp cleans up the environment override file
func (p *InitializeK0s) CleanUp() {
	h := p.leader
	if len(h.Environment) > 0 {
		if err := h.Configurer.CleanupServiceEnvironment(h, h.K0sServiceName()); err != nil {
			log.Warnf("%s: failed to clean up service environment: %s", h, err.Error())
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
		p.SetProp("dynamic-config", true)
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
	if err = h.Exec(cmd); err != nil {
		return err
	}

	h.Metadata.K0sInstalled = true

	if len(h.Environment) > 0 {
		log.Infof("%s: updating service environment", h)
		if err := h.Configurer.UpdateServiceEnvironment(h, h.K0sServiceName(), h.Environment); err != nil {
			return err
		}
	}

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

	if p.Config.Spec.K0s.DynamicConfig {
		if err := retry.Timeout(context.TODO(), retry.DefaultTimeout, node.K0sDynamicConfigReadyFunc(h)); err != nil {
			return fmt.Errorf("dynamic config reconciliation failed: %w", err)
		}
	}

	h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
	h.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version

	if id, err := p.Config.Spec.K0s.GetClusterID(h); err == nil {
		p.Config.Spec.K0s.Metadata.ClusterID = id
		p.SetProp("clusterID", id)
	}

	return nil
}
