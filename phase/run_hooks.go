package phase

import (
    "context"
    "fmt"

    "golang.org/x/text/cases"
    "golang.org/x/text/language"

    "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
    "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

// RunHooks is a generic phase to execute per-host lifecycle hooks for a given
// action and stage (e.g. action "apply" with stage "before").
type RunHooks struct {
    GenericPhase

    // Action is the lifecycle action: apply, backup, reset, install, etc.
    Action string
    // Stage is the timing within the action: before or after.
    Stage string

    hosts cluster.Hosts
}

// Title returns a human-friendly phase title.
func (p *RunHooks) Title() string {
    titler := cases.Title(language.AmericanEnglish)
    if p.Stage == "" && p.Action == "" {
        return "Run Hooks"
    }
    if p.Stage == "" {
        return fmt.Sprintf("Run %s Hooks", titler.String(p.Action))
    }
    return fmt.Sprintf("Run %s %s Hooks", titler.String(p.Stage), titler.String(p.Action))
}

// Prepare collects the hosts from the config.
func (p *RunHooks) Prepare(c *v1beta1.Cluster) error {
    p.Config = c
    // Include all hosts; runHooks is a no-op on hosts without matching hooks.
    p.hosts = c.Spec.Hosts
    return nil
}

// ShouldRun returns true when there are hosts.
func (p *RunHooks) ShouldRun() bool {
    return len(p.hosts) > 0
}

// Run executes the hooks on all selected hosts.
func (p *RunHooks) Run(ctx context.Context) error {
    return p.runHooks(ctx, p.Action, p.Stage, p.hosts...)
}
