package phase

import "github.com/k0sproject/k0sctl/config"

// GenericPhase is a basic phase which gets a config via prepare, sets it into p.Config
type GenericPhase struct {
	Config *config.Cluster
}

// Prepare the phase
func (p *GenericPhase) Prepare(c *config.Cluster) error {
	p.Config = c
	return nil
}
