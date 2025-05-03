package phase

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/logrusorgru/aurora"
	log "github.com/sirupsen/logrus"
)

// NoWait is used by various phases to decide if node ready state should be waited for or not
var NoWait bool

// Force is used by various phases to attempt a forced installation
var Force bool

// Colorize is an instance of "aurora", used to colorize the output
var Colorize = aurora.NewAurora(false)

// Phase represents a runnable phase which can be added to Manager.
type Phase interface {
	Run(context.Context) error
	Title() string
}

// Phases is a slice of Phases
type Phases []Phase

// Index returns the index of the first occurrence matching the given phase title or -1 if not found
func (p Phases) Index(title string) int {
	for i, phase := range p {
		if phase.Title() == title {
			return i
		}
	}
	return -1
}

// Remove removes the first occurrence of a phase with the given title
func (p *Phases) Remove(title string) {
	i := p.Index(title)
	if i == -1 {
		return
	}
	*p = append((*p)[:i], (*p)[i+1:]...)
}

// InsertAfter inserts a phase after the first occurrence of a phase with the given title
func (p *Phases) InsertAfter(title string, phase Phase) {
	i := p.Index(title)
	if i == -1 {
		return
	}
	*p = append((*p)[:i+1], append(Phases{phase}, (*p)[i+1:]...)...)
}

// InsertBefore inserts a phase before the first occurrence of a phase with the given title
func (p *Phases) InsertBefore(title string, phase Phase) {
	i := p.Index(title)
	if i == -1 {
		return
	}
	*p = append((*p)[:i], append(Phases{phase}, (*p)[i:]...)...)
}

// Replace replaces the first occurrence of a phase with the given title
func (p *Phases) Replace(title string, phase Phase) {
	i := p.Index(title)
	if i == -1 {
		return
	}
	(*p)[i] = phase
}

type withconfig interface {
	Title() string
	Prepare(*v1beta1.Cluster) error
}

type conditional interface {
	ShouldRun() bool
}

type withcleanup interface {
	CleanUp()
}

type withmanager interface {
	SetManager(*Manager)
}

type withDryRun interface {
	DryRun() error
}

type withBeforeHook interface {
	BeforeHook() error
}

type withAfterHook interface {
	AfterHook() error
}

type withSuccessHook interface {
	SuccessHook() error
}

type withFailureHook interface {
	FailureHook() error
}

// Manager executes phases to construct the cluster
type Manager struct {
	phases            Phases
	Config            *v1beta1.Cluster
	Concurrency       int
	ConcurrentUploads int
	DryRun            bool
	Writer            io.Writer

	dryMessages map[string][]string
	dryMu       sync.Mutex
}

// NewManager creates a new Manager
func NewManager(config *v1beta1.Cluster) (*Manager, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	return &Manager{Config: config, Writer: os.Stdout}, nil
}

// AddPhase adds a Phase to Manager
func (m *Manager) AddPhase(p ...Phase) {
	m.phases = append(m.phases, p...)
}

// SetPhases sets the list of phases
func (m *Manager) SetPhases(p Phases) {
	m.phases = p
}

type errorfunc func() error

// DryMsg prints a message in dry-run mode
func (m *Manager) DryMsg(host fmt.Stringer, msg string) {
	m.dryMu.Lock()
	defer m.dryMu.Unlock()
	if m.dryMessages == nil {
		m.dryMessages = make(map[string][]string)
	}
	var key string
	if host == nil {
		key = "local"
	} else {
		key = host.String()
	}
	m.dryMessages[key] = append(m.dryMessages[key], msg)
}

// Wet runs the first given function when not in dry-run mode. The second function will be
// run when in dry-mode and the message will be displayed. Any error returned from the
// functions will be returned and will halt the operation.
func (m *Manager) Wet(host fmt.Stringer, msg string, funcs ...errorfunc) error {
	if !m.DryRun {
		if len(funcs) > 0 && funcs[0] != nil {
			return funcs[0]()
		}
		return nil
	}

	m.DryMsg(host, msg)

	if m.DryRun && len(funcs) == 2 && funcs[1] != nil {
		return funcs[1]()
	}

	return nil
}

// Run executes all the added Phases in order
func (m *Manager) Run(ctx context.Context) error {
	var ran []Phase
	var result error

	defer func() {
		if m.DryRun {
			if len(m.dryMessages) == 0 {
				fmt.Fprintln(m.Writer, Colorize.BrightGreen("dry-run: no cluster state altering actions would be performed"))
				return
			}

			fmt.Fprintln(m.Writer, Colorize.BrightRed("dry-run: cluster state altering actions would be performed:"))
			for host, msgs := range m.dryMessages {
				fmt.Fprintln(m.Writer, Colorize.BrightRed("dry-run:"), Colorize.Bold(fmt.Sprintf("* %s :", host)))
				for _, msg := range msgs {
					fmt.Println(Colorize.BrightRed("dry-run:"), Colorize.Red(" -"), msg)
				}
			}
			return
		}
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

		if err := ctx.Err(); err != nil {
			result = fmt.Errorf("context canceled before entering phase %q: %w", title, err)
			return result
		}

		if p, ok := p.(withmanager); ok {
			p.SetManager(m)
		}

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

		if hp, ok := p.(withBeforeHook); ok {
			log.Debugf("running before hook for phase '%s'", p.Title())
			if err := hp.BeforeHook(); err != nil {
				log.Debugf("before hook failed '%s'", err.Error())
				return err
			}
		}

		text := Colorize.Green("==> Running phase: %s").String()
		log.Infof(text, title)

		if dp, ok := p.(withDryRun); ok && m.DryRun {
			if err := dp.DryRun(); err != nil {
				return err
			}
			continue
		}

		result = p.Run(ctx)
		ran = append(ran, p)

		if result != nil {
			if hp, ok := p.(withFailureHook); ok {
				log.Debugf("running failure hook for phase '%s'", p.Title())
				if herr := hp.FailureHook(); herr != nil {
					result = fmt.Errorf("phase failed: '%s' (hook failed: %w)", result.Error(), herr)
				}
			}
			return result
		}

		if hp, ok := p.(withAfterHook); ok {
			log.Debugf("running after hook for phase '%s'", p.Title())
			if herr := hp.AfterHook(); herr != nil {
				result = fmt.Errorf("phase failed: '%s' (hook failed: %w)", result.Error(), herr)
				break
			}
		}

		if result != nil {
			return result
		}
	}

	return nil
}
