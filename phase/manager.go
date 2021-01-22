package phase

import (
	"github.com/k0sproject/k0sctl/config"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
)

type phase interface {
	Run() error
	Title() string
}

type withconfig interface {
	Prepare(*config.Cluster) error
}

type conditional interface {
	ShouldRun() bool
}

// beforehook receives the phase title as an argument because of reasons.
type beforehook interface {
	Before(string) error
}

type afterhook interface {
	After(error) error
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
		title := p.Title()

		if p, ok := p.(withconfig); ok {
			if err := p.Prepare(m.Config); err != nil {
				return err
			}
		}

		if p, ok := p.(conditional); ok {
			if !p.ShouldRun() {
				continue
			}
		}

		if p, ok := p.(beforehook); ok {
			if err := p.Before(title); err != nil {
				log.Debugf("before hook failed '%s'", err.Error())
				return err
			}
		}

		text := aurora.Green("==> Running phase: %s").String()
		log.Infof(text, title)
		result := p.Run()

		if p, ok := p.(afterhook); ok {
			if err := p.After(result); err != nil {
				log.Debugf("after hook failed: '%s' (phase result: %s)", err.Error(), result)
				return err
			}
		}

		if result != nil {
			return result
		}
	}

	return nil
}
