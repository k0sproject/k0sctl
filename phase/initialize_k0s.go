package phase

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
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
	if leader.Metadata.K0sRunningVersion == "" {
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

	log.Infof("%s: installing k0s controller", h)
	if err := h.Exec(h.K0sInstallCommand()); err != nil {
		return err
	}

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
	if err := h.WaitK0sServiceRunning(); err != nil {
		return err
	}

	port := 6443
	if ap := p.Config.Spec.K0s.Config.Spec.API.Port; ap != 0 {
		port = ap
	}
	log.Infof("%s: waiting for kubernetes api to respond", h)
	if err := h.WaitKubeAPIReady(port); err != nil {
		return err
	}

	h.Metadata.K0sRunningVersion = p.Config.Spec.K0s.Version
	h.Metadata.K0sBinaryVersion = p.Config.Spec.K0s.Version

	if id, err := p.Config.Spec.K0s.GetClusterID(h); err == nil {
		p.Config.Spec.K0s.Metadata.ClusterID = id
		p.SetProp("clusterID", id)
	}

	return nil
}
