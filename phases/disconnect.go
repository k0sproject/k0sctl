package phases

import (
	"reflect"
	"sync"

	log "github.com/sirupsen/logrus"
)

type disconnectable interface {
	Disconnect()
	String() string
}

// Disconnect phase implementation
type Disconnect struct {
	hosts []disconnectable
}

// Prepare digs out the hosts from the config
func (p *Disconnect) Prepare(config interface{}) error {
	r := reflect.ValueOf(config).Elem()
	spec := r.FieldByName("Spec").Elem()
	hosts := spec.FieldByName("Hosts")
	for i := 0; i < hosts.Len(); i++ {
		h := hosts.Index(i)
		if !h.Elem().FieldByName("Connection").IsNil() {
			h := hosts.Index(i).Interface().(disconnectable)
			p.hosts = append(p.hosts, h)
		}
	}

	return nil
}

// ShouldRun is true when there are hosts that need to be connected
func (p *Disconnect) ShouldRun() bool {
	return len(p.hosts) > 0
}

// CleanUp does nothing
func (p *Disconnect) CleanUp() {}

// Title for the phase
func (p *Disconnect) Title() string {
	return "Close Connection"
}

// Run disconnects from all the hosts
func (p *Disconnect) Run() error {
	var wg sync.WaitGroup
	wg.Add(len(p.hosts))

	for _, h := range p.hosts {
		go func(h disconnectable) {
			h.Disconnect()
			log.Infof("%s: disconnected", h)
			wg.Done()
		}(h)
	}

	wg.Wait()

	return nil
}
