package phase

import (
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
)

// NoWait is used by various phases to decide if node ready state should be waited for or not
var NoWait bool
var Colorize = aurora.NewAurora(false)

type phase interface {
	Run() error
	Title() string
}

type withconfig interface {
	Title() string
	Prepare(*v1beta1.Cluster) error
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

type propsetter interface {
	SetProp(string, interface{})
}

type withcleanup interface {
	CleanUp()
}

// Manager executes phases to construct the cluster
type Manager struct {
	phases []phase
	Config *v1beta1.Cluster
}

// AddPhase adds a Phase to Manager
func (m *Manager) AddPhase(p ...phase) {
	m.phases = append(m.phases, p...)
}

// Run executes all the added Phases in order
func (m *Manager) Run() error {
	var ran []phase
	var result error

	defer func() {
		if result != nil {
			for _, p := range ran {
				if c, ok := p.(withcleanup); ok {
					log.Infof(Colorize.Red("* Running clean-up for phase: %s").String(), p.Title())
					c.CleanUp()
				}
			}
		}
	}()

	for _, p := range m.phases {
		title := p.Title()

		if p, ok := p.(withconfig); ok {
			log.Debugf("Preparing phase '%s'", p.Title())
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

		if p, ok := p.(propsetter); ok {
			if m.Config.Spec.K0s.Metadata.ClusterID != "" {
				p.SetProp("clusterID", m.Config.Spec.K0s.Metadata.ClusterID)
			}
		}

		text := Colorize.Green("==> Running phase: %s").String()
		log.Infof(text, title)
		result = p.Run()
		ran = append(ran, p)

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
