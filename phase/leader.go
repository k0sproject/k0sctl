package phase

import (
	"errors"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// Leader clears the config hosts and only leaves one leader
type Leader struct {
	GenericPhase
}

func (p *Leader) Title() string {
	return "Determine cluster leader"
}

func (p *Leader) Prepare(c *v1beta1.Cluster) error {
	l := c.Spec.K0sLeader()

	if l == nil {
		return errors.New("could not find a cluster leader")
	}

	c.Spec.Hosts = cluster.Hosts{l}
	return nil
}

func (p *Leader) ShouldRun() bool {
	return false
}

func (p *Leader) Run() error {
	return errors.New("this should never be run")
}
