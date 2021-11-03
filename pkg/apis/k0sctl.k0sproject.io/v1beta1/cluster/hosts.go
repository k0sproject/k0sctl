package cluster

import (
	"fmt"
	"strings"
	"sync"
)

//Hosts are destnation hosts
type Hosts []*Host

// First returns the first host
func (hosts *Hosts) First() *Host {
	if len(*hosts) == 0 {
		return nil
	}
	return (*hosts)[0]
}

// Last returns the last host
func (hosts *Hosts) Last() *Host {
	c := len(*hosts) - 1

	if c < 0 {
		return nil
	}

	return (*hosts)[c]
}

// Find returns the first matching Host. The finder function should return true for a Host matching the criteria.
func (hosts *Hosts) Find(filter func(h *Host) bool) *Host {
	for _, h := range *hosts {
		if filter(h) {
			return (h)
		}
	}
	return nil
}

// Filter returns a filtered list of Hosts. The filter function should return true for hosts matching the criteria.
func (hosts *Hosts) Filter(filter func(h *Host) bool) Hosts {
	result := make(Hosts, 0, len(*hosts))

	for _, h := range *hosts {
		if filter(h) {
			result = append(result, h)
		}
	}

	return result
}

// WithRole returns a ltered list of Hosts that have the given role
func (hosts *Hosts) WithRole(s string) Hosts {
	return hosts.Filter(func(h *Host) bool {
		return h.Role == s
	})
}

// Controllers returns hosts with the role "controller"
func (hosts *Hosts) Controllers() Hosts {
	return hosts.Filter(func(h *Host) bool { return h.IsController() })
}

// Workers returns hosts with the role "worker"
func (hosts *Hosts) Workers() Hosts {
	return hosts.WithRole("worker")
}

// ParallelEach runs a function (or multiple functions chained) on every Host parallelly.
// Any errors will be concatenated and returned.
func (hosts *Hosts) ParallelEach(filter ...func(h *Host) error) error {
	var wg sync.WaitGroup
	var errors []string
	type erritem struct {
		address string
		err     error
	}
	ec := make(chan erritem, 1)

	for _, f := range filter {
		wg.Add(len(*hosts))

		for _, h := range *hosts {
			go func(h *Host) {
				ec <- erritem{h.String(), f(h)}
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
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed on %d hosts:\n - %s", len(errors), strings.Join(errors, "\n - "))
	}

	return nil
}
