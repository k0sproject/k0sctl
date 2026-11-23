package action

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
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
	// ErrWriter receives the per-host failures while following. Defaults
	// to os.Stderr.
	ErrWriter io.Writer
	// Concurrency limits how many hosts are connected to and read from at
	// once. 0 is unlimited. Following streams from all hosts at once.
	Concurrency int

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
	connected, errs := connectLogHosts(ctx, hosts, l.Concurrency, connect)
	if len(connected) == 0 {
		return errors.Join(errs...)
	}

	// Per-host failures are shown on ErrWriter while following, because
	// the final error is only shown after the user interrupts. Without
	// --follow the failures are only returned, so they aren't shown twice.
	errWriter := l.ErrWriter
	if errWriter == nil {
		errWriter = os.Stderr
	}
	errOut := &syncWriter{w: errWriter}
	report := func(err error) {
		if l.Follow {
			_, _ = fmt.Fprintf(errOut, "k0sctl: %v\n", err)
		}
	}
	for _, err := range errs {
		report(err)
	}

	out := &syncWriter{w: l.Writer}
	var labels map[*cluster.Host]string
	if len(hosts) > 1 {
		labels = logLabels(hosts)
	}

	if l.Lines > 0 {
		fetchErr, writeErr := l.printRecent(ctx, connected, out, labels, fetch)
		if writeErr != nil {
			// Nothing more can be shown when the output fails.
			return errors.Join(append(errs, fetchErr, writeErr)...)
		}
		if fetchErr != nil {
			// Streaming may still work where reading the history didn't.
			report(fmt.Errorf("reading the latest logs failed: %w", fetchErr))
			errs = append(errs, fetchErr)
		}
	}

	if !l.Follow {
		return errors.Join(errs...)
	}

	// When the output fails, stop all streams. Otherwise quiet hosts keep
	// the command waiting although nothing can be shown anymore.
	streamCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	out.fail = func(err error) { cancel(fmt.Errorf("write logs: %w", err)) }

	err = connected.ParallelEach(streamCtx, func(ctx context.Context, h *cluster.Host) error {
		var err error
		if labels == nil {
			err = stream(ctx, h, out)
		} else {
			pw := &prefixWriter{w: out, prefix: labels[h] + ": "}
			err = errors.Join(stream(ctx, h, pw), pw.Flush())
		}
		// The other hosts may keep streaming, so tell the user now that
		// this one stopped. An interrupt stops all of them, no need to
		// report that.
		if err != nil && ctx.Err() == nil {
			report(fmt.Errorf("%s: streaming logs failed: %w", h, err))
		}
		return err
	})
	if ctx.Err() == nil && streamCtx.Err() != nil {
		// The output failed. The errors from the stopped streams only
		// repeat that.
		err = context.Cause(streamCtx)
	}
	return errors.Join(append(errs, err)...)
}

// forEachLimited calls fn for 0..n-1 in parallel, with at most limit calls
// running at once. A limit of 0 or less is unlimited.
func forEachLimited(n, limit int, fn func(i int)) {
	if limit <= 0 || limit > n {
		limit = n
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			fn(i)
		}()
	}
	wg.Wait()
}

// logLabels returns the label to prefix the log lines of each host with. The
// label is the address, or the address and port when hosts share an address.
func logLabels(hosts cluster.Hosts) map[*cluster.Host]string {
	count := make(map[string]int, len(hosts))
	for _, h := range hosts {
		count[h.Address()]++
	}
	labels := make(map[*cluster.Host]string, len(hosts))
	for _, h := range hosts {
		labels[h] = h.Address()
		if count[h.Address()] > 1 {
			labels[h] = hostPort(h)
		}
	}
	return labels
}

func hostPort(h *cluster.Host) string {
	switch {
	case h.SSH != nil:
		return net.JoinHostPort(h.SSH.Address, strconv.Itoa(h.SSH.Port))
	case h.WinRM != nil:
		return net.JoinHostPort(h.WinRM.Address, strconv.Itoa(h.WinRM.Port))
	case h.OpenSSH != nil && h.OpenSSH.Port != nil:
		return net.JoinHostPort(h.OpenSSH.Address, strconv.Itoa(*h.OpenSSH.Port))
	}
	return h.Address()
}

// connectLogHosts connects to the hosts in parallel, at most limit at once.
// It returns the hosts that connected and an error for each host that didn't.
func connectLogHosts(ctx context.Context, hosts cluster.Hosts, limit int, connect func(context.Context, *cluster.Host) error) (cluster.Hosts, []error) {
	errs := make([]error, len(hosts))
	forEachLimited(len(hosts), limit, func(i int) {
		if err := connect(ctx, hosts[i]); err != nil {
			errs[i] = fmt.Errorf("%s: %w", hosts[i], err)
		}
	})

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

// printRecent fetches the latest lines from the hosts in parallel, at most
// l.Concurrency at once. It prints them one host after the other, so the
// lines of a host stay together. It returns the per-host fetch failures and the output write error separately,
// because only the fetch failures can be recovered from.
func (l Logs) printRecent(ctx context.Context, hosts cluster.Hosts, out io.Writer, labels map[*cluster.Host]string, fetch func(context.Context, *cluster.Host, int) ([]string, error)) (fetchErr, writeErr error) {
	results := make([][]string, len(hosts))
	errs := make([]error, len(hosts))
	forEachLimited(len(hosts), l.Concurrency, func(i int) {
		results[i], errs[i] = fetch(ctx, hosts[i], l.Lines)
	})

	var failed []error
	for i, h := range hosts {
		if errs[i] != nil {
			failed = append(failed, fmt.Errorf("%s: %w", h, errs[i]))
			continue
		}
		for _, line := range trimTrailingEmpty(results[i]) {
			if labels != nil {
				line = labels[h] + ": " + line
			}
			if _, err := fmt.Fprintln(out, line); err != nil {
				return errors.Join(failed...), fmt.Errorf("write logs: %w", err)
			}
		}
	}
	return errors.Join(failed...), nil
}

// selectLogHosts returns the hosts matching the given addresses and role. An
// address matches every host with that address, because hosts can share an
// address and use different ports. An address that isn't in the config is an
// error, so that a typo doesn't silently show nothing.
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
		seen := make(map[*cluster.Host]struct{}, len(addresses))
		for _, a := range addresses {
			matches := hosts.Filter(func(h *cluster.Host) bool { return h.Address() == a })
			if len(matches) == 0 {
				missing = append(missing, a)
				continue
			}
			for _, h := range matches {
				if _, ok := seen[h]; ok {
					continue
				}
				seen[h] = struct{}{}
				selected = append(selected, h)
			}
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
	// fail is called when a write fails, if set.
	fail func(error)
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, err := s.w.Write(p)
	if err != nil && s.fail != nil {
		s.fail(err)
	}
	return n, err
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
