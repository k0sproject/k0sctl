package phase

import "github.com/k0sproject/k0sctl/config"

type GenericPhase struct {
	Config *config.Cluster
}

func (p *GenericPhase) Prepare(c *config.Cluster) error {
	p.Config = c
	return nil
}

func (p *GenericPhase) ShouldRun() bool {
	return true
}
