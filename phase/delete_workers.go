package phase

import (
	"encoding/json"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

type kubectlGetNodesNodeMetadata struct {
	Name string `json:"name"`
}

type kubectlGetNodesNode struct {
	Metadata *kubectlGetNodesNodeMetadata `json:"metadata"`
}

type kubectlGetNodes struct {
	Items []*kubectlGetNodesNode `json:"items"`
}

// DeleteWorkers drain and delete k0s worker nodes
type DeleteWorkers struct {
	GenericPhase
	workersToDelete []string
	leader          *cluster.Host
}

// Title for the phase
func (p *DeleteWorkers) Title() string {
	return "Delete workers"
}

// Prepare the phase
func (p *DeleteWorkers) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	output, err := p.leader.ExecOutput(p.leader.Configurer.KubectlCmdf("get nodes -l role.kubernetes.io/control-plane!=true -o json"), exec.Sudo(p.leader))
	if err != nil {
		return err
	}

	workers := kubectlGetNodes{}
	if err := json.Unmarshal([]byte(output), &workers); err != nil {
		log.Warnf("%s: failed to decode kubectl get nodes output: %s", p.leader, err.Error())
		return nil
	}

	for _, worker := range workers.Items {
		foundWorker := p.Config.Spec.Hosts.Find(func(h *cluster.Host) bool {
			return h.Metadata.Hostname == worker.Metadata.Name
		})

		if foundWorker == nil {
			p.workersToDelete = append(p.workersToDelete, worker.Metadata.Name)
		}
	}

	return nil
}

// ShouldRun is true when there are workers to delete
func (p *DeleteWorkers) ShouldRun() bool {
	return len(p.workersToDelete) > 0
}

// Run the phase
func (p *DeleteWorkers) Run() error {
	for _, worker := range p.workersToDelete {
		log.Infof("%s: draining worker %s...", p.leader, worker)

		if err := p.leader.DrainNode(&cluster.Host{
			Metadata: cluster.HostMetadata{
				Hostname: worker,
			},
		}); err != nil {
			return err
		}

		log.Infof("%s: deleting worker %s...", p.leader, worker)

		if err := p.leader.DeleteNode(&cluster.Host{
			Metadata: cluster.HostMetadata{
				Hostname: worker,
			},
		}); err != nil {
			return err
		}

		log.Infof("%s: worker %s deleted, please terminate the host manually", p.leader, worker)
	}

	return nil
}
