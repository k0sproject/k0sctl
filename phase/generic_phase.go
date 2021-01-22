package phase

import (
	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/config"
)

// GenericPhase is a basic phase which gets a config via prepare, sets it into p.Config
type GenericPhase struct {
	analytics.Phase
	Config *config.Cluster
}

func (p *GenericPhase) GetConfig() *config.Cluster {
	return p.Config
}

// Prepare the phase
func (p *GenericPhase) Prepare(c *config.Cluster) error {
	p.Config = c
	return nil
}
