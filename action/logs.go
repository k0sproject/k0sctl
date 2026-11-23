package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	log "github.com/sirupsen/logrus"
)

// DefaultLogLines is how many of the latest log lines Logs shows per host by
// default.
const DefaultLogLines = 100

// Logs shows the logs of the k0s service on the cluster hosts.
type Logs struct {
	Config *v1beta1.Cluster
	// Addresses limits the hosts to the ones with these addresses.
	Addresses []string
	// Role limits the hosts to "controller" or "worker" hosts.
	Role string
	// Lines is how many of the latest lines to show per host.
	Lines int
	// Follow keeps streaming new log lines until the context is cancelled.
	Follow bool
	Writer io.Writer
	// ErrWriter receives the per-host failures while following, because
	// the final error is only shown after the user interrupts. Defaults to
	// os.Stderr.
	ErrWriter io.Writer

	// These are replaced in tests, by default they use the host's
	// connection and init system.
	connect func(ctx context.Context, h *cluster.Host) error
	fetch   func(ctx context.Context, h *cluster.Host, lines int) ([]string, error)
	stream  func(ctx context.Context, h *cluster.Host, w io.Writer) error
}

// Run shows the logs.
func (l Logs) Run(ctx context.Context) error {
	hosts, err := selectLogHosts(l.Config.Spec.Hosts, l.Addresses, l.Role)
	if err != nil {
		return err
	}

	connect, fetch, stream := l.connect, l.fetch, l.stream
	if connect == nil {
		connect = connectHost
	}
	if fetch == nil {
		fetch = fetchServiceLogs
	}
	if stream == nil {
		stream = streamServiceLogs
	}

	defer func() {
		for _, h := range hosts {
			if h.Client != nil {
				h.Disconnect()
			}
		}
	}()

	// Show the logs of the hosts that connected, and report the others.
	connected, errs := connectLogHosts(ctx, hosts, connect)
	if len(connected) == 0 {
		return errors.Join(errs...)
	}
	for _, err := range errs {
		l.report(err)
	}

	out := &syncWriter{w: l.Writer}
	prefix := len(hosts) > 1

	if l.Lines > 0 {
		if err := l.printRecent(ctx, connected, out, prefix, fetch); err != nil {
			// Streaming may still work where reading the history didn't.
			l.report(fmt.Errorf("reading the latest logs failed: %w", err))
			errs = append(errs, err)
		}
	}

	if !l.Follow {
		return errors.Join(errs...)
	}

	err = connected.ParallelEach(ctx, func(ctx context.Context, h *cluster.Host) error {
		if !prefix {
			return stream(ctx, h, out)
		}
		pw := &prefixWriter{w: out, prefix: h.Address() + ": "}
		return errors.Join(stream(ctx, h, pw), pw.Flush())
	})
	return errors.Join(append(errs, err)...)
}

// report shows a per-host failure on ErrWriter while following. Without
// --follow the failure is only returned, so that it isn't shown twice.
func (l Logs) report(err error) {
	if !l.Follow {
		return
	}
	w := l.ErrWriter
	if w == nil {
		w = os.Stderr
	}
	_, _ = fmt.Fprintf(w, "k0sctl: %v\n", err)
}

// connectLogHosts connects to the hosts in parallel. It returns the hosts
// that connected and an error for each host that didn't.
func connectLogHosts(ctx context.Context, hosts cluster.Hosts, connect func(context.Context, *cluster.Host) error) (cluster.Hosts, []error) {
	errs := make([]error, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := connect(ctx, h); err != nil {
				errs[i] = fmt.Errorf("%s: %w", h, err)
			}
		}()
	}
	wg.Wait()

	connected := make(cluster.Hosts, 0, len(hosts))
	var failed []error
	for i, h := range hosts {
		if errs[i] != nil {
			failed = append(failed, errs[i])
			continue
		}
		connected = append(connected, h)
	}
	return connected, failed
}

// printRecent fetches the latest lines from every host in parallel and prints
// them one host after the other, so the lines of a host stay together.
func (l Logs) printRecent(ctx context.Context, hosts cluster.Hosts, out io.Writer, prefix bool, fetch func(context.Context, *cluster.Host, int) ([]string, error)) error {
	results := make([][]string, len(hosts))
	errs := make([]error, len(hosts))
	var wg sync.WaitGroup
	for i, h := range hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i], errs[i] = fetch(ctx, h, l.Lines)
		}()
	}
	wg.Wait()

	var failed []error
	for i, h := range hosts {
		if errs[i] != nil {
			failed = append(failed, fmt.Errorf("%s: %w", h, errs[i]))
			continue
		}
		for _, line := range trimTrailingEmpty(results[i]) {
			if prefix {
				line = h.Address() + ": " + line
			}
			if _, err := fmt.Fprintln(out, line); err != nil {
				return fmt.Errorf("write logs: %w", err)
			}
		}
	}
	return errors.Join(failed...)
}

// selectLogHosts returns the hosts matching the given addresses and role. An
// address that isn't in the config is an error, so that a typo doesn't
// silently show nothing.
func selectLogHosts(hosts cluster.Hosts, addresses []string, role string) (cluster.Hosts, error) {
	switch role {
	case "":
	case "controller":
		hosts = hosts.Controllers()
	case "worker":
		hosts = hosts.Filter(func(h *cluster.Host) bool {
			return h.Role == "worker" || h.Role == "controller+worker" || h.Role == "single"
		})
	default:
		return nil, fmt.Errorf("invalid role %q, expected controller or worker", role)
	}

	if len(addresses) > 0 {
		var missing []string
		selected := make(cluster.Hosts, 0, len(addresses))
		seen := make(map[string]struct{}, len(addresses))
		for _, a := range addresses {
			if _, ok := seen[a]; ok {
				continue
			}
			seen[a] = struct{}{}
			h := hosts.Find(func(h *cluster.Host) bool { return h.Address() == a })
			if h == nil {
				missing = append(missing, a)
				continue
			}
			selected = append(selected, h)
		}
		if len(missing) > 0 {
			return nil, fmt.Errorf("no matching host with address %s", strings.Join(missing, ", "))
		}
		hosts = selected
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("no hosts matched")
	}
	return hosts, nil
}

func connectHost(ctx context.Context, h *cluster.Host) error {
	log.Debugf("%s: connecting", h)
	if err := h.Connect(ctx); err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return nil
}

func fetchServiceLogs(ctx context.Context, h *cluster.Host, lines int) ([]string, error) {
	svc, err := h.Sudo().Service(h.K0sServiceName())
	if err != nil {
		return nil, fmt.Errorf("get service %s: %w", h.K0sServiceName(), err)
	}
	return svc.Logs(ctx, lines)
}

func streamServiceLogs(ctx context.Context, h *cluster.Host, w io.Writer) error {
	svc, err := h.Sudo().Service(h.K0sServiceName())
	if err != nil {
		return fmt.Errorf("get service %s: %w", h.K0sServiceName(), err)
	}
	return svc.StreamLogs(ctx, w)
}

func trimTrailingEmpty(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// syncWriter serializes writes from several hosts.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// prefixWriter prefixes every complete line with prefix and writes it as one
// write, so that lines streamed from several hosts don't get mixed up.
type prefixWriter struct {
	w      io.Writer
	prefix string
	buf    []byte
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.buf = append(p.buf, b...)
	for {
		i := bytes.IndexByte(p.buf, '\n')
		if i < 0 {
			break
		}
		line := make([]byte, 0, len(p.prefix)+i+1)
		line = append(line, p.prefix...)
		line = append(line, p.buf[:i+1]...)
		p.buf = p.buf[i+1:]
		if _, err := p.w.Write(line); err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

// Flush writes out a last line that didn't end with a newline.
func (p *prefixWriter) Flush() error {
	if len(p.buf) == 0 {
		return nil
	}
	_, err := p.w.Write([]byte(p.prefix + string(p.buf) + "\n"))
	p.buf = nil
	return err
}
