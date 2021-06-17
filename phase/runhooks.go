package phase

import (
	"fmt"
	"strings"
	"sync"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
)

var _ phase = &RunHooks{}

// RunHooks phase runs a set of hooks configured for the host
type RunHooks struct {
	Action string
	Stage  string

	steps map[*cluster.Host][]string
}

// Title for the phase
func (p *RunHooks) Title() string {
	return fmt.Sprintf("Run %s %s Hooks", strings.Title(p.Stage), strings.Title(p.Action))
}

// Prepare digs out the hosts with steps from the config
func (p *RunHooks) Prepare(config *config.Cluster) error {
	p.steps = make(map[*cluster.Host][]string)
	for _, h := range config.Spec.Hosts {
		if len(h.Hooks) > 0 {
			p.steps[h] = h.Hooks.ForActionAndStage(p.Action, p.Stage)
		}
	}

	return nil
}

// ShouldRun is true when there are hosts that need to be connected
func (p *RunHooks) ShouldRun() bool {
	return len(p.steps) > 0
}

// Run does all the prep work on the hosts in parallel
func (p *RunHooks) Run() error {
	var wg sync.WaitGroup
	var errors []string
	type erritem struct {
		host string
		err  error
	}
	ec := make(chan erritem, 1)

	wg.Add(len(p.steps))

	for h, steps := range p.steps {
		go func(h *cluster.Host, steps []string) {
			for _, s := range steps {
				err := h.Exec(s)
				if err != nil {
					ec <- erritem{h.String(), err}
					return // do not exec remaining steps if one fails
				}
			}

			ec <- erritem{h.String(), nil}
		}(h, steps)
	}

	go func() {
		for e := range ec {
			if e.err != nil {
				errors = append(errors, fmt.Sprintf("%s: %s", e.host, e.err.Error()))
			}
			wg.Done()
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("failed on %d hosts:\n - %s", len(errors), strings.Join(errors, "\n - "))
	}

	return nil
}
