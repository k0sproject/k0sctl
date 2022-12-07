package phase

import (
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// GenericPhase is a basic phase which gets a config via prepare, sets it into p.Config
type GenericPhase struct {
	analytics.Phase
	Config *v1beta1.Cluster

	manager *Manager
}

// GetConfig is an accessor to phase Config
func (p *GenericPhase) GetConfig() *v1beta1.Cluster {
	return p.Config
}

// Prepare the phase
func (p *GenericPhase) Prepare(c *v1beta1.Cluster) error {
	p.Config = c
	return nil
}

// SetManager adds a reference to the phase manager
func (p *GenericPhase) SetManager(m *Manager) {
	p.manager = m
}

func (p *GenericPhase) parallelDo(hosts cluster.Hosts, funcs ...func(h *cluster.Host) error) error {
	if p.manager.Concurrency == 0 {
		return hosts.ParallelEach(funcs...)
	}
	return hosts.BatchedParallelEach(p.manager.Concurrency, funcs...)
}

func (p *GenericPhase) parallelDoUpload(hosts cluster.Hosts, funcs ...func(h *cluster.Host) error) error {
	if p.manager.Concurrency == 0 {
		return hosts.ParallelEach(funcs...)
	}
	return hosts.BatchedParallelEach(p.manager.ConcurrentUploads, funcs...)
}
