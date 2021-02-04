package phase

import (
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/k0sproject/k0sctl/config"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
)

// NoWait is used by various phases to decide if node ready state should be waited for or not
var NoWait bool

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

type propsetter interface {
	SetProp(string, interface{})
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
	start := time.Now()
	if err := analytics.Client.Publish("apply-start", map[string]interface{}{}); err != nil {
		return err
	}

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

		if p, ok := p.(propsetter); ok {
			if m.Config.Spec.K0s.Metadata.ClusterID != "" {
				p.SetProp("clusterID", m.Config.Spec.K0s.Metadata.ClusterID)
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
			_ = analytics.Client.Publish("apply-failure", map[string]interface{}{"phase": p.Title(), "clusterID": m.Config.Spec.K0s.Metadata.ClusterID})
			return result
		}
	}

	return analytics.Client.Publish("apply-success", map[string]interface{}{"duration": time.Since(start), "clusterID": m.Config.Spec.K0s.Metadata.ClusterID})
}
