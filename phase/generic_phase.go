package phase

import (
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// GenericPhase is a basic phase which gets a config via prepare, sets it into p.Config
type GenericPhase struct {
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

// Wet is a shorthand for manager.Wet
func (p *GenericPhase) Wet(host fmt.Stringer, msg string, funcs ...errorfunc) error {
	return p.manager.Wet(host, msg, funcs...)
}

// IsWet returns true if manager is in dry-run mode
func (p *GenericPhase) IsWet() bool {
	return !p.manager.DryRun
}

// DryMsg is a shorthand for manager.DryMsg
func (p *GenericPhase) DryMsg(host fmt.Stringer, msg string) {
	p.manager.DryMsg(host, msg)
}

// DryMsgf is a shorthand for manager.DryMsg + fmt.Sprintf
func (p *GenericPhase) DryMsgf(host fmt.Stringer, msg string, args ...any) {
	p.manager.DryMsg(host, fmt.Sprintf(msg, args...))
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
