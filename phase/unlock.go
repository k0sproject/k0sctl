package phase

import (
	"context"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	log "github.com/sirupsen/logrus"
)

// Unlock acquires an exclusive k0sctl lock on hosts
type Unlock struct {
	GenericPhase
	Cancel func()
}

// Prepare the phase
func (p *Unlock) Prepare(c *v1beta1.Cluster) error {
	p.Config = c
	if p.Cancel == nil {
		p.Cancel = func() {
			log.Fatalf("cancel function not defined")
		}
	}
	return nil
}

// Title for the phase
func (p *Unlock) Title() string {
	return "Release exclusive host lock"
}

// Run the phase
func (p *Unlock) Run(_ context.Context) error {
	p.Cancel()
	return nil
}
