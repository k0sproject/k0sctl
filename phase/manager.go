package phase

import (
	"context"
	"fmt"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
)

// NoWait is used by various phases to decide if node ready state should be waited for or not
var NoWait bool
var Colorize = aurora.NewAurora(false)

type Phase interface {
	Run() error
	Title() string
}

type Getter interface {
	Value(any) any
}

type withinitializer interface {
	Initialize(Getter) error
}

type withconfig interface {
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

type ConfigKey struct{}

// Manager executes phases to construct the cluster
type Manager struct {
	phases  []Phase
	Config  *v1beta1.Cluster
	context context.Context
}

func NewManager(ctx context.Context) *Manager {
	m := &Manager{}
	m.context = ctx
	if cfg, ok := ctx.Value(ConfigKey{}).(*v1beta1.Cluster); ok {
		m.Config = cfg
	}
	return m
}

func (m *Manager) Value(name any) any {
	return m.context.Value(name)
}

// AddPhase adds a Phase to Manager
func (m *Manager) AddPhase(p ...Phase) {
	m.phases = append(m.phases, p...)
}

func (m *Manager) Index(title string) int {
	for i, p := range m.phases {
		if p.Title() == title {
			return i
		}
	}
	return -1
}

func (m *Manager) AddPhaseBefore(title string, b Phase) error {
	idx := m.Index(title)

	if idx < 0 {
		return fmt.Errorf("couldn't find phase %s", title)
	}

	m.phases = append(m.phases[:idx], append([]Phase{b}, m.phases[idx:]...)...)

	return nil
}

// Run executes all the added Phases in order
func (m *Manager) Run() error {
	var ran []Phase
	var result error

	defer func() {
		if result != nil {
			for _, p := range ran {
				title := p.Title()
				if c, ok := p.(withcleanup); ok {
					log.Infof(Colorize.Red("* Running clean-up for phase: %s").String(), title)
					c.CleanUp()
				}
			}
		}
	}()

	for _, p := range m.phases {
		title := p.Title()

		if p, ok := p.(withinitializer); ok {
			log.Debugf("Initializing phase '%s'", title)
			if err := p.Initialize(m); err != nil {
				return err
			}
		}

		if p, ok := p.(withconfig); ok {
			log.Debugf("Preparing phase '%s'", title)
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
