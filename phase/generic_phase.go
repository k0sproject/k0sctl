package phase

import (
    "context"
    "fmt"

    "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
    "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// GenericPhase is a basic phase which gets a config via prepare, sets it into p.Config
type GenericPhase struct {
	Config *v1beta1.Cluster

	manager *Manager
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

// Wet is a shorthand for manager.Wet
func (p *GenericPhase) Wet(host fmt.Stringer, msg string, funcs ...errorfunc) error {
    return p.manager.Wet(host, msg, funcs...)
}

// IsWet returns true when not in dry-run mode (i.e., wet mode)
func (p *GenericPhase) IsWet() bool {
    return !p.manager.DryRun
}

// DryMsg is a shorthand for manager.DryMsg
func (p *GenericPhase) DryMsg(host fmt.Stringer, msg string) {
	p.manager.DryMsg(host, msg)
}

// DryMsgf is a shorthand for manager.DryMsg + fmt.Sprintf
func (p *GenericPhase) DryMsgf(host fmt.Stringer, msg string, args ...any) {
	p.manager.DryMsg(host, fmt.Sprintf(msg, args...))
}

// SetManager adds a reference to the phase manager
func (p *GenericPhase) SetManager(m *Manager) {
	p.manager = m
}

func (p *GenericPhase) parallelDo(ctx context.Context, hosts cluster.Hosts, funcs ...func(context.Context, *cluster.Host) error) error {
	if p.manager.Concurrency == 0 {
		return hosts.ParallelEach(ctx, funcs...)
	}
	return hosts.BatchedParallelEach(ctx, p.manager.Concurrency, funcs...)
}

func (p *GenericPhase) parallelDoUpload(ctx context.Context, hosts cluster.Hosts, funcs ...func(context.Context, *cluster.Host) error) error {
    if p.manager.Concurrency == 0 {
        return hosts.ParallelEach(ctx, funcs...)
    }
    return hosts.BatchedParallelEach(ctx, p.manager.ConcurrentUploads, funcs...)
}

// runHooks executes hooks for the provided hosts honoring the given context.
func (p *GenericPhase) runHooks(ctx context.Context, action, stage string, hosts ...*cluster.Host) error {
    return p.parallelDo(ctx, hosts, func(_ context.Context, h *cluster.Host) error {
        if !p.IsWet() {
            // In dry-run, list each hook command that would be executed.
            cmds := h.Hooks.ForActionAndStage(action, stage)
            for _, cmd := range cmds {
                p.DryMsgf(h, "run %s %s hook: %q", stage, action, cmd)
            }
            return nil
        }

        if err := h.RunHooks(ctx, action, stage); err != nil {
            return fmt.Errorf("running hooks failed: %w", err)
        }

        return nil
    })
}
