package action

import (
	"fmt"

	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

type Kubeconfig struct {
	Config               *v1beta1.Cluster
	Concurrency          int
	KubeconfigAPIAddress string

	Kubeconfig string
}

func (k *Kubeconfig) Run() error {
	if k.Config == nil {
		return fmt.Errorf("config is nil")
	}

	c := k.Config

	// Change so that the internal config has only single controller host as we
	// do not need to connect to all nodes
	c.Spec.Hosts = cluster.Hosts{c.Spec.K0sLeader()}
	manager := phase.Manager{Config: k.Config, Concurrency: k.Concurrency}

	manager.AddPhase(
		&phase.Connect{},
		&phase.DetectOS{},
		&phase.GetKubeconfig{APIAddress: k.KubeconfigAPIAddress},
		&phase.Disconnect{},
	)

	return manager.Run()
}
