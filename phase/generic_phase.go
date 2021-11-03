package phase

import (
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
)

// GenericPhase is a basic phase which gets a config via prepare, sets it into p.Config
type GenericPhase struct {
	analytics.Phase
	Config *v1beta1.Cluster
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
