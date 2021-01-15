package phase

import (
	"fmt"
	"reflect"
	"time"

	"github.com/k0sproject/k0sctl/analytics"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
	event "gopkg.in/segmentio/analytics-go.v3"
)

type phase interface {
	Run() error
	Title() string
	Prepare(interface{}) error
	ShouldRun() bool
	CleanUp()
}

// Manager executes phases to construct the cluster
type Manager struct {
	phases       []phase
	config       interface{}
	IgnoreErrors bool
	SkipCleanup  bool
}

// NewManager constructs new phase manager
func NewManager(config interface{}) *Manager {
	phaseMgr := &Manager{
		config: config,
	}

	return phaseMgr
}

// AddPhases add multiple phases to manager in one call
func (m *Manager) AddPhases(phases ...phase) {
	m.phases = append(m.phases, phases...)
}

// AddPhase adds a Phase to Manager
func (m *Manager) AddPhase(p phase) {
	m.phases = append(m.phases, p)
}

// Run executes all the added Phases in order
func (m *Manager) Run() error {
	for _, p := range m.phases {
		fmt.Println("RUNNING", len(m.phases))
		fmt.Println("preparing phase ", p.Title())
		log.Debugf("preparing phase '%s'", p.Title())
		err := p.Prepare(m.config)
		if err != nil {
			return err
		}

		if !p.ShouldRun() {
			log.Debugf("skipping phase '%s'", p.Title())
			continue
		}

		text := aurora.Green("==> Running phase: %s").String()
		log.Infof(text, p.Title())
		if e, ok := interface{}(p).(Eventable); ok {
			start := time.Now()
			r := reflect.ValueOf(m.config).Elem()
			props := event.Properties{
				"kind":        r.FieldByName("Kind").String(),
				"api_version": r.FieldByName("APIVersion").String(),
			}
			fmt.Println("**********HERE************")

			err := p.Run()

			duration := time.Since(start)
			props["duration"] = duration.Seconds()
			for k, v := range e.GetEventProperties() {
				props[k] = v
			}
			if err != nil {
				props["success"] = false
				analytics.TrackEvent(p.Title(), props)
				if !m.IgnoreErrors {
					return err
				}
			}
			props["success"] = true
			analytics.TrackEvent(p.Title(), props)

		} else {
			err := p.Run()
			if err != nil && !m.IgnoreErrors {
				return err
			}
			if !m.SkipCleanup {
				defer p.CleanUp()
			}
		}
	}

	return nil
}
