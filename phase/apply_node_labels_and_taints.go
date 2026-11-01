package phase

import (
	"context"
	"fmt"
	"strings"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	log "github.com/sirupsen/logrus"
)

// ApplyNodeLabelsAndTaints sets and removes the node labels and taints given
// in the host configuration through the Kubernetes API.
type ApplyNodeLabelsAndTaints struct {
	GenericPhase
	leader *cluster.Host
	hosts  cluster.Hosts
}

// Title for the phase
func (p *ApplyNodeLabelsAndTaints) Title() string {
	return "Apply node labels and taints"
}

// Prepare the phase
func (p *ApplyNodeLabelsAndTaints) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return !h.Reset && h.Role != "controller" && (!h.NodeLabels.IsEmpty() || !h.NodeTaints.IsEmpty())
	})

	return nil
}

// ShouldRun is true when there are node labels or taints to manage
func (p *ApplyNodeLabelsAndTaints) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *ApplyNodeLabelsAndTaints) Run(ctx context.Context) error {
	for _, h := range p.hosts {
		if err := p.configure(ctx, h); err != nil {
			return fmt.Errorf("%s: apply node labels and taints: %w", h, err)
		}
	}

	return nil
}

func (p *ApplyNodeLabelsAndTaints) configure(ctx context.Context, h *cluster.Host) error {
	var (
		labels map[string]string
		taints []string
	)
	readNode := func(_ context.Context) error {
		var err error
		if labels, err = p.leader.NodeLabelValues(h); err != nil {
			return err
		}
		taints, err = p.leader.Taints(h)
		return err
	}

	if !p.IsWet() {
		// A node that is not in the cluster yet gets all of its labels and taints.
		if err := readNode(ctx); err != nil {
			labels, taints = map[string]string{}, nil
		}
	} else if err := retry.WithDefaultTimeout(ctx, readNode); err != nil {
		// The node may still be registering when the apply was run with --no-wait.
		return err
	}

	applyLabels, deleteLabels := h.NodeLabels.Changes(labels)
	if len(applyLabels) > 0 || len(deleteLabels) > 0 {
		msg := describeNodeChanges("labels", applyLabels, deleteLabels)
		if err := p.Wet(h, msg, func() error {
			log.Infof("%s: %s", h, msg)
			return p.leader.LabelNode(h, applyLabels, deleteLabels)
		}); err != nil {
			return err
		}
	}

	applyTaints, deleteTaints := h.NodeTaints.Changes(taints)
	if len(applyTaints) > 0 || len(deleteTaints) > 0 {
		msg := describeNodeChanges("taints", applyTaints, deleteTaints)
		if err := p.Wet(h, msg, func() error {
			log.Infof("%s: %s", h, msg)
			return p.leader.TaintNode(h, applyTaints, deleteTaints)
		}); err != nil {
			return err
		}
	}

	return nil
}

func describeNodeChanges(kind string, apply, del []string) string {
	var parts []string
	if len(apply) > 0 {
		parts = append(parts, "set "+kind+" "+strings.Join(apply, ", "))
	}
	if len(del) > 0 {
		parts = append(parts, "remove "+kind+" "+strings.Join(del, ", "))
	}
	return strings.Join(parts, " and ")
}
