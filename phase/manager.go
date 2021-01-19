package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
)

type phase interface {
	Run() error
	Title() string
	Prepare(*config.Cluster) error
	ShouldRun() bool
}

// Manager executes phases to construct the cluster
type Manager struct {
	phases []phase
	Config *config.Cluster
}

// AddPhase adds a Phase to Manager
func (m *Manager) AddPhase(p ...phase) {
	m.phases = append(m.phases, p...)
}

// Run executes all the added Phases in order
func (m *Manager) Run() error {
	for _, p := range m.phases {
		log.Debugf("preparing phase '%s'", p.Title())
		err := p.Prepare(m.Config)
		if err != nil {
			return err
		}

		if !p.ShouldRun() {
			log.Debugf("skipping phase '%s'", p.Title())
			continue
		}

		text := aurora.Green("==> Running phase: %s").String()
		log.Infof(text, p.Title())
		if err := p.Run(); err != nil {
			return err
		}
	}

	return nil
}
