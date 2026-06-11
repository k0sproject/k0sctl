package phase

import (
	"bytes"
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
)

// ApplyManifests is a phase that applies additional manifests to the cluster
type ApplyManifests struct {
	GenericPhase
	leader *cluster.Host
}

// Title for the phase
func (p *ApplyManifests) Title() string {
	return "Apply additional manifests"
}

// Prepare the phase
func (p *ApplyManifests) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.leader = p.Config.Spec.K0sLeader()

	return nil
}

// ShouldRun is true when there are additional manifests to apply
func (p *ApplyManifests) ShouldRun() bool {
	return len(p.Config.Metadata.Manifests) > 0
}

// Run the phase
func (p *ApplyManifests) Run(ctx context.Context) error {
	for name, content := range p.Config.Metadata.Manifests {
		if err := p.apply(ctx, name, content); err != nil {
			return err
		}
	}

	return nil
}

func (p *ApplyManifests) apply(ctx context.Context, name string, content []byte) error {
	if !p.IsWet() {
		p.DryMsgf(p.leader, "apply manifest %s (%d bytes)", name, len(content))
		return nil
	}

	log.Infof("%s: apply manifest %s (%d bytes)", p.leader, name, len(content))
	kubectlCmd := p.leader.Configurer.KubectlCmdf(p.leader, p.leader.K0sDataDir(), "apply -f -")
	var stdout, stderr bytes.Buffer

	proc := p.leader.Sudo().Proc(kubectlCmd)
	proc.Stdin = bytes.NewReader(content)
	proc.Stdout = &stdout
	proc.Stderr = &stderr
	waiter, err := proc.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to run apply for manifest %s: %w", name, err)
	}
	if err := waiter.Wait(); err != nil {
		return fmt.Errorf("kubectl apply failed for manifest %s: %w (stderr: %s)", name, err, stderr.String())
	}
	log.Infof("%s: kubectl apply: %s", p.leader, stdout.String())
	return nil
}
