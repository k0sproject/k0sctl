package phase

import (
	"strings"

	"github.com/k0sproject/k0sctl/config"
	log "github.com/sirupsen/logrus"
)

// BasicPhase is a phase which has all the basic functionality like Title and default implementations for Prepare and ShouldRun
type BasicPhase struct {
	Config *config.ClusterConfig
}

// HostSelectPhase is a phase where hosts are collected before running to see if it's necessary to run the phase at all in ShouldRun
type HostSelectPhase struct {
	BasicPhase
	Hosts config.Hosts
}

// Prepare rceives the cluster config and stores it to the phase's config field
func (p *BasicPhase) Prepare(c interface{}) error {
	p.Config = c.(*config.ClusterConfig)
	return nil
}

// ShouldRun for BasicPhases is always true
func (p *BasicPhase) ShouldRun() bool {
	return true
}

// CleanUp basic implementation
func (p *BasicPhase) CleanUp() {}

// Title default implementation
func (p *HostSelectPhase) Title() string {
	return ""
}

// Run default implementation
func (p *HostSelectPhase) Run() error {
	return nil
}

// Prepare HostSelectPhase implementation which runs the supplied HostFilterFunc to populate the phase's hosts field
func (p *HostSelectPhase) Prepare(c interface{}) error {
	p.Config = c.(*config.ClusterConfig)
	hosts := p.Config.Spec.Hosts.Filter(p.HostFilterFunc)
	p.Hosts = hosts
	return nil
}

// ShouldRun HostSelectPhase default implementation which returns true if there are hosts that matched the HostFilterFunc
func (p *HostSelectPhase) ShouldRun() bool {
	return len(p.Hosts) > 0
}

// HostFilterFunc default implementation, matches all hosts
func (p *HostSelectPhase) HostFilterFunc(host *config.Host) bool {
	return true
}

// Eventable interface
type Eventable interface {
	GetEventProperties() map[string]interface{}
}

// Analytics struct
type Analytics struct {
	EventProperties map[string]interface{}
}

// GetEventProperties returns analytic event properties
func (p *Analytics) GetEventProperties() map[string]interface{} {
	return p.EventProperties
}

// Error collects multiple error into one as we execute many phases in parallel
// for many hosts.
type Error struct {
	Errors []error
}

// AddError adds new error to the collection
func (e *Error) AddError(err error) {
	e.Errors = append(e.Errors, err)
}

// Count returns the current count of errors
func (e *Error) Count() int {
	return len(e.Errors)
}

// Error returns the combined stringified error
func (e *Error) Error() string {
	messages := []string{}
	for _, err := range e.Errors {
		messages = append(messages, err.Error())
	}
	return strings.Join(messages, "\n")
}

// RunParallelOnHosts runs a function parallelly on the listed hosts
func RunParallelOnHosts(hosts config.Hosts, c *config.ClusterConfig, action func(h *config.Host, config *config.ClusterConfig) error) error {
	return hosts.ParallelEach(func(h *config.Host) error {
		err := action(h, c)
		if err != nil {
			log.Error(err.Error())
		}
		return err
	})
}
