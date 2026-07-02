package display

import (
	"log/slog"
	"slices"
	"sync"
)

// ringSize is how many recent records are kept per host for failure
// forensics and live log tails.
const ringSize = 100

type ring struct {
	events [ringSize]Event
	next   int
	count  int
}

func (r *ring) add(ev Event) {
	r.events[r.next] = ev
	r.next = (r.next + 1) % ringSize
	if r.count < ringSize {
		r.count++
	}
}

// tail returns up to n most recent events, oldest first.
func (r *ring) tail(n int) []Event {
	if n > r.count {
		n = r.count
	}
	out := make([]Event, 0, n)
	for i := r.count - n; i < r.count; i++ {
		out = append(out, r.events[(r.next-r.count+i+ringSize*2)%ringSize])
	}
	return out
}

// rings keeps per-host buffers of recent events regardless of the visible
// log level, plus the set of hosts that have reported errors.
type rings struct {
	mu     sync.Mutex
	hosts  map[string]*ring
	failed []string
}

func newRings() *rings {
	return &rings{hosts: make(map[string]*ring)}
}

func (rs *rings) add(ev Event) {
	if ev.Host == "" {
		return
	}
	rs.mu.Lock()
	defer rs.mu.Unlock()
	r, ok := rs.hosts[ev.Host]
	if !ok {
		r = &ring{}
		rs.hosts[ev.Host] = r
	}
	r.add(ev)
	if ev.Level >= slog.LevelError && !slices.Contains(rs.failed, ev.Host) {
		rs.failed = append(rs.failed, ev.Host)
	}
}

// failures returns the hosts that logged errors and up to n recent events
// for each, oldest first.
func (rs *rings) failures(n int) map[string][]Event {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	if len(rs.failed) == 0 {
		return nil
	}
	out := make(map[string][]Event, len(rs.failed))
	for _, host := range rs.failed {
		out[host] = rs.hosts[host].tail(n)
	}
	return out
}

// tail returns up to n recent events for a host, oldest first.
func (rs *rings) tail(host string, n int) []Event {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	r, ok := rs.hosts[host]
	if !ok {
		return nil
	}
	return r.tail(n)
}
