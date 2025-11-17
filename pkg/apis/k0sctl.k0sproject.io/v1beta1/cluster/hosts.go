package cluster

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Hosts are destnation hosts
type Hosts []*Host

func (hosts Hosts) Validate() error {
	if len(hosts) == 0 {
		return fmt.Errorf("at least one host required")
	}

	if len(hosts) > 1 {
		hostmap := make(map[string]struct{}, len(hosts))
		for idx, h := range hosts {
			if err := h.Validate(); err != nil {
				return fmt.Errorf("host #%d: %v", idx+1, err)
			}
			if h.Role == "single" {
				return fmt.Errorf("%d hosts defined but includes a host with role 'single': %s", len(hosts), h)
			}
			if _, ok := hostmap[h.String()]; ok {
				return fmt.Errorf("%s: is not unique", h)
			}
			hostmap[h.String()] = struct{}{}
		}
	}

	if len(hosts.Controllers()) < 1 {
		return fmt.Errorf("no hosts with a controller role defined")
	}

	return nil
}

// Resolve runs Host.Resolve for each host.
func (hosts Hosts) Resolve(baseDir string) error {
	for _, h := range hosts {
		if err := h.Resolve(baseDir); err != nil {
			return err
		}
	}
	return nil
}

// First returns the first host
func (hosts Hosts) First() *Host {
	if len(hosts) == 0 {
		return nil
	}
	return (hosts)[0]
}

// Last returns the last host
func (hosts Hosts) Last() *Host {
	c := len(hosts) - 1

	if c < 0 {
		return nil
	}

	return hosts[c]
}

// Find returns the first matching Host. The finder function should return true for a Host matching the criteria.
func (hosts Hosts) Find(filter func(h *Host) bool) *Host {
	for _, h := range hosts {
		if filter(h) {
			return (h)
		}
	}
	return nil
}

// Filter returns a filtered list of Hosts. The filter function should return true for hosts matching the criteria.
func (hosts Hosts) Filter(filter func(h *Host) bool) Hosts {
	result := make(Hosts, 0, len(hosts))

	for _, h := range hosts {
		if filter(h) {
			result = append(result, h)
		}
	}

	return result
}

// WithRole returns a ltered list of Hosts that have the given role
func (hosts Hosts) WithRole(s string) Hosts {
	return hosts.Filter(func(h *Host) bool {
		return h.Role == s
	})
}

// Controllers returns hosts with the role "controller"
func (hosts Hosts) Controllers() Hosts {
	return hosts.Filter(func(h *Host) bool { return h.IsController() })
}

// Workers returns hosts with the role "worker"
func (hosts Hosts) Workers() Hosts {
	return hosts.WithRole("worker")
}

// Each runs a function (or multiple functions chained) on every Host.
func (hosts Hosts) Each(ctx context.Context, filters ...func(context.Context, *Host) error) error {
	for _, filter := range filters {
		for _, h := range hosts {
			if err := ctx.Err(); err != nil {
				return fmt.Errorf("error from context: %w", err)
			}
			if err := filter(ctx, h); err != nil {
				return err
			}
		}
	}

	return nil
}

// ParallelEach runs a function (or multiple functions chained) on every Host parallelly.
// Any errors will be concatenated and returned.
func (hosts Hosts) ParallelEach(ctx context.Context, filters ...func(context.Context, *Host) error) error {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errors []string

	for _, filter := range filters {
		for _, h := range hosts {
			wg.Add(1)
			go func(h *Host) {
				defer wg.Done()
				if err := ctx.Err(); err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("error from context: %v", err))
					mu.Unlock()
					return
				}
				if err := filter(ctx, h); err != nil {
					mu.Lock()
					errors = append(errors, fmt.Sprintf("%s: %s", h.String(), err.Error()))
					mu.Unlock()
				}
			}(h)
		}
		wg.Wait()
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed on %d hosts:\n - %s", len(errors), strings.Join(errors, "\n - "))
	}

	return nil
}

// BatchedParallelEach runs a function (or multiple functions chained) on every Host parallelly in groups of batchSize hosts.
func (hosts Hosts) BatchedParallelEach(ctx context.Context, batchSize int, filter ...func(context.Context, *Host) error) error {
	for i := 0; i < len(hosts); i += batchSize {
		end := i + batchSize
		if end > len(hosts) {
			end = len(hosts)
		}
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("error from context: %w", err)
		}
		if err := hosts[i:end].ParallelEach(ctx, filter...); err != nil {
			return err
		}
	}

	return nil
}
