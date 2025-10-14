package action

import (
	"context"

	"github.com/k0sproject/k0sctl/phase"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

type Kubeconfig struct {
	// Manager is the phase manager
	Manager              *phase.Manager
	KubeconfigAPIAddress string
	KubeconfigUser       string
	KubeconfigCluster    string

	Kubeconfig string
}

func (k *Kubeconfig) Run(ctx context.Context) error {
	// Change so that the internal config has only single controller host as we
	// do not need to connect to all nodes
	k.Manager.Config.Spec.Hosts = cluster.Hosts{k.Manager.Config.Spec.K0sLeader()}

    k.Manager.AddPhase(
        &phase.Connect{},
        &phase.DetectOS{},
        &phase.GetKubeconfig{APIAddress: k.KubeconfigAPIAddress, User: k.KubeconfigUser, Cluster: k.KubeconfigCluster},
        &phase.Disconnect{},
    )

	return k.Manager.Run(ctx)
}
