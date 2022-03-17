package phase

import (
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
)

var _ phase = &RunHooks{}

// RunHooks phase runs a set of hooks configured for the host
type RunHooks struct {
	Action string
	Stage  string
	hosts  cluster.Hosts
}

// Title for the phase
func (p *RunHooks) Title() string {
	titler := cases.Title(language.AmericanEnglish)
	return fmt.Sprintf("Run %s %s Hooks", titler.String(p.Stage), titler.String(p.Action))
}

// Prepare digs out the hosts with steps from the config
func (p *RunHooks) Prepare(config *v1beta1.Cluster) error {
	p.hosts = config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return len(h.Hooks.ForActionAndStage(p.Action, p.Stage)) > 0
	})

	return nil
}

// ShouldRun is true when there are hosts that need to be connected
func (p *RunHooks) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run does all the prep work on the hosts in parallel
func (p *RunHooks) Run() error {
	return p.hosts.ParallelEach(p.runHooksForHost)
}

func (p *RunHooks) runHooksForHost(h *cluster.Host) error {
	steps := h.Hooks.ForActionAndStage(p.Action, p.Stage)
	for _, s := range steps {
		err := h.Exec(s)
		if err != nil {
			return err
		}
	}
	return nil
}
