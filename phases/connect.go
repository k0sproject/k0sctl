package phases

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	retry "github.com/avast/retry-go"
	"github.com/k0sproject/rig/exec"
	log "github.com/sirupsen/logrus"
)

type connectable interface {
	Connect() error
	String() string
	Exec(cmd string, opts ...exec.Option) error
}

// Connect connects to each of the hosts
type Connect struct {
	hosts []connectable
	// phase.BasicPhase
}

// Prepare digs out the hosts from the config
func (p *Connect) Prepare(config interface{}) error {
	r := reflect.ValueOf(config).Elem()
	spec := r.FieldByName("Spec").Elem()
	hosts := spec.FieldByName("Hosts")
	for i := 0; i < hosts.Len(); i++ {
		if h, ok := hosts.Index(i).Interface().(connectable); ok {
			p.hosts = append(p.hosts, h)
		}
	}

	// fmt.Println("hosts", spew.Sdump(p.hosts))
	// for _, h := range p.hosts {
	// 	t := reflect.TypeOf(h)
	// 	for i := 0; i < t.NumMethod(); i++ {
	// 		method := t.Method(i)
	// 		fmt.Println(method.Name)
	// 	}
	// }
	return nil
}

// ShouldRun is true when there are hosts that need to be connected
func (p *Connect) ShouldRun() bool {
	return len(p.hosts) > 0
}

// CleanUp does nothing
func (p *Connect) CleanUp() {}

// Title for the phase
func (p *Connect) Title() string {
	return "Open Remote Connection"
}

// Run connects to all the hosts in parallel
func (p *Connect) Run() error {
	var wg sync.WaitGroup
	var errors []string
	type erritem struct {
		address string
		err     error
	}
	ec := make(chan erritem, 1)

	wg.Add(len(p.hosts))

	for _, h := range p.hosts {
		go func(h connectable) {
			ec <- erritem{h.String(), p.connectHost(h)}
		}(h)
	}

	go func() {
		for e := range ec {
			if e.err != nil {
				errors = append(errors, fmt.Sprintf("%s: %s", e.address, e.err.Error()))
			}
			wg.Done()
		}
	}()

	wg.Wait()

	if len(errors) > 0 {
		return fmt.Errorf("failed on %d hosts:\n - %s", len(errors), strings.Join(errors, "\n - "))
	}

	return nil
}

const retries = 60

func (p *Connect) connectHost(h connectable) error {
	err := retry.Do(
		func() error {
			return h.Connect()
		},
		retry.OnRetry(
			func(n uint, err error) {
				log.Errorf("%s: attempt %d of %d.. failed to connect: %s", h, n+1, retries, err.Error())
			},
		),
		retry.RetryIf(
			func(err error) bool {
				return !strings.Contains(err.Error(), "no supported methods remain")
			},
		),
		retry.DelayType(retry.CombineDelay(retry.FixedDelay, retry.RandomDelay)),
		retry.MaxJitter(time.Second*2),
		retry.Delay(time.Second*3),
		retry.Attempts(retries),
	)

	if err != nil {
		return err
	}

	return p.testConnection(h)
}

func (p *Connect) testConnection(h connectable) error {
	log.Infof("%s: testing connection", h)

	if err := h.Exec("echo"); err != nil {
		return err
	}

	return nil
}
