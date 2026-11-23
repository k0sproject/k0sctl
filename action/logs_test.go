package action

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/protocol/ssh"
	"github.com/stretchr/testify/require"
)

func logTestHost(address, role string) *cluster.Host {
	return &cluster.Host{
		Role:            role,
		CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: address}},
	}
}

func logTestHostPort(address string, port int, role string) *cluster.Host {
	h := logTestHost(address, role)
	h.SSH.Port = port
	return h
}

func logTestCluster(hosts ...*cluster.Host) *v1beta1.Cluster {
	return &v1beta1.Cluster{Spec: &cluster.Spec{Hosts: hosts}}
}

func addresses(hosts cluster.Hosts) []string {
	out := make([]string, 0, len(hosts))
	for _, h := range hosts {
		out = append(out, h.Address())
	}
	return out
}

func TestSelectLogHosts_FiltersByRoleAndAddress(t *testing.T) {
	hosts := cluster.Hosts{
		logTestHost("10.0.0.1", "controller"),
		logTestHost("10.0.0.2", "controller+worker"),
		logTestHost("10.0.0.3", "worker"),
	}

	got, err := selectLogHosts(hosts, nil, "")
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"}, addresses(got))

	got, err = selectLogHosts(hosts, nil, "controller")
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, addresses(got))

	got, err = selectLogHosts(hosts, nil, "worker")
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.2", "10.0.0.3"}, addresses(got))

	got, err = selectLogHosts(hosts, []string{"10.0.0.3", "10.0.0.1"}, "")
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.3", "10.0.0.1"}, addresses(got))

	got, err = selectLogHosts(hosts, []string{"10.0.0.1", "10.0.0.1"}, "")
	require.NoError(t, err)
	require.Equal(t, []string{"10.0.0.1"}, addresses(got))
}

func TestSelectLogHosts_AddressMatchesAllPorts(t *testing.T) {
	a := logTestHostPort("127.0.0.1", 9022, "controller")
	b := logTestHostPort("127.0.0.1", 9023, "worker")
	hosts := cluster.Hosts{a, b, logTestHost("10.0.0.3", "worker")}

	got, err := selectLogHosts(hosts, []string{"127.0.0.1", "127.0.0.1"}, "")
	require.NoError(t, err)
	require.Equal(t, cluster.Hosts{a, b}, got)
}

func TestSelectLogHosts_RejectsUnknownAddressAndRole(t *testing.T) {
	hosts := cluster.Hosts{logTestHost("10.0.0.1", "controller"), logTestHost("10.0.0.3", "worker")}

	_, err := selectLogHosts(hosts, []string{"10.0.0.1", "10.0.0.9"}, "")
	require.ErrorContains(t, err, "10.0.0.9")

	_, err = selectLogHosts(hosts, []string{"10.0.0.3"}, "controller")
	require.ErrorContains(t, err, "10.0.0.3")

	_, err = selectLogHosts(hosts, nil, "single")
	require.ErrorContains(t, err, "invalid role")
}

func fakeLogs(cfg *v1beta1.Cluster, w io.Writer, logs map[string][]string, streams map[string]string) Logs {
	return Logs{
		Config: cfg,
		Writer: w,
		Lines:  DefaultLogLines,
		connect: func(context.Context, *cluster.Host) error {
			return nil
		},
		fetch: func(_ context.Context, h *cluster.Host, _ int) ([]string, error) {
			lines, ok := logs[h.Address()]
			if !ok {
				return nil, errors.New("no journal")
			}
			return lines, nil
		},
		stream: func(_ context.Context, h *cluster.Host, w io.Writer) error {
			// Write in odd chunks to check that lines are put back together.
			s := streams[h.Address()]
			for len(s) > 0 {
				n := min(3, len(s))
				if _, err := w.Write([]byte(s[:n])); err != nil {
					return err
				}
				s = s[n:]
			}
			return nil
		},
	}
}

func TestLogs_SingleHostIsNotPrefixed(t *testing.T) {
	var out bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"))
	l := fakeLogs(cfg, &out, map[string][]string{"10.0.0.1": {"one", "two", ""}}, nil)

	require.NoError(t, l.Run(context.Background()))
	require.Equal(t, "one\ntwo\n", out.String())
}

func TestLogs_MultipleHostsArePrefixedAndGrouped(t *testing.T) {
	var out bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"), logTestHost("10.0.0.3", "worker"))
	l := fakeLogs(cfg, &out, map[string][]string{
		"10.0.0.1": {"c1", "c2"},
		"10.0.0.3": {"w1"},
	}, nil)

	require.NoError(t, l.Run(context.Background()))
	require.Equal(t, "10.0.0.1: c1\n10.0.0.1: c2\n10.0.0.3: w1\n", out.String())
}

func TestLogs_FailedHostDoesNotHideTheOthers(t *testing.T) {
	var out bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"), logTestHost("10.0.0.3", "worker"))
	l := fakeLogs(cfg, &out, map[string][]string{"10.0.0.3": {"w1"}}, nil)

	err := l.Run(context.Background())
	require.ErrorContains(t, err, "no journal")
	require.Equal(t, "10.0.0.3: w1\n", out.String())
}

func TestLogs_FollowStreamsWholePrefixedLines(t *testing.T) {
	var out bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"), logTestHost("10.0.0.3", "worker"))
	l := fakeLogs(cfg, &out, map[string][]string{"10.0.0.1": {}, "10.0.0.3": {}}, map[string]string{
		"10.0.0.1": "controller line\nunterminated",
		"10.0.0.3": "worker line one\nworker line two\n",
	})
	l.Follow = true

	require.NoError(t, l.Run(context.Background()))

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	require.ElementsMatch(t, []string{
		"10.0.0.1: controller line",
		"10.0.0.1: unterminated",
		"10.0.0.3: worker line one",
		"10.0.0.3: worker line two",
	}, lines)
}

func TestLogs_FollowWithoutHistory(t *testing.T) {
	var out bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"))
	l := fakeLogs(cfg, &out, nil, map[string]string{"10.0.0.1": "new\n"})
	l.Lines = 0
	l.Follow = true

	require.NoError(t, l.Run(context.Background()))
	require.Equal(t, "new\n", out.String())
}

func TestLogs_FailedConnectionDoesNotHideTheOthers(t *testing.T) {
	var out bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"), logTestHost("10.0.0.3", "worker"))
	l := fakeLogs(cfg, &out, map[string][]string{"10.0.0.1": {"c1"}, "10.0.0.3": {"w1"}}, nil)
	l.connect = func(_ context.Context, h *cluster.Host) error {
		if h.Address() == "10.0.0.1" {
			return errors.New("unreachable")
		}
		return nil
	}

	err := l.Run(context.Background())
	require.ErrorContains(t, err, "unreachable")
	require.Equal(t, "10.0.0.3: w1\n", out.String())
}

func TestLogs_FollowReportsHistoryFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"))
	l := fakeLogs(cfg, &out, nil, map[string]string{"10.0.0.1": "new\n"})
	l.Follow = true
	l.ErrWriter = &errOut

	err := l.Run(context.Background())
	require.ErrorContains(t, err, "no journal")
	require.Contains(t, errOut.String(), "10.0.0.1")
	require.Contains(t, errOut.String(), "no journal")
	require.Equal(t, "new\n", out.String())
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestLogs_WriteFailureIsReturned(t *testing.T) {
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"))
	l := fakeLogs(cfg, failingWriter{}, map[string][]string{"10.0.0.1": {"one"}}, nil)

	require.ErrorContains(t, l.Run(context.Background()), "disk full")
}

func TestLogs_HostsSharingAnAddressArePrefixedWithPort(t *testing.T) {
	var out bytes.Buffer
	cfg := logTestCluster(logTestHostPort("127.0.0.1", 9022, "controller"), logTestHostPort("127.0.0.1", 9023, "worker"))
	l := fakeLogs(cfg, &out, nil, nil)
	l.fetch = func(_ context.Context, h *cluster.Host, _ int) ([]string, error) {
		return []string{strconv.Itoa(h.SSH.Port)}, nil
	}

	require.NoError(t, l.Run(context.Background()))
	require.Equal(t, "127.0.0.1:9022: 9022\n127.0.0.1:9023: 9023\n", out.String())
}

func TestLogs_FollowReportsStreamFailureWhileOthersStream(t *testing.T) {
	var out, errOut bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"), logTestHost("10.0.0.3", "worker"))
	l := fakeLogs(cfg, &out, map[string][]string{"10.0.0.1": {}, "10.0.0.3": {}}, nil)
	l.Follow = true

	reported := make(chan struct{})
	l.stream = func(_ context.Context, h *cluster.Host, _ io.Writer) error {
		if h.Address() == "10.0.0.1" {
			return errors.New("connection lost")
		}
		// Keep streaming until the failure of the other host is reported.
		<-reported
		return nil
	}
	l.ErrWriter = writerFunc(func(p []byte) (int, error) {
		n, err := errOut.Write(p)
		close(reported)
		return n, err
	})

	err := l.Run(context.Background())
	require.ErrorContains(t, err, "connection lost")
	require.Contains(t, errOut.String(), "10.0.0.1")
	require.Contains(t, errOut.String(), "connection lost")
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

func TestLogs_FollowStopsOnHistoryWriteFailure(t *testing.T) {
	var errOut bytes.Buffer
	cfg := logTestCluster(logTestHost("10.0.0.1", "controller"))
	l := fakeLogs(cfg, failingWriter{}, map[string][]string{"10.0.0.1": {"one"}}, nil)
	l.Follow = true
	l.ErrWriter = &errOut
	l.stream = func(context.Context, *cluster.Host, io.Writer) error {
		t.Error("streaming started after the output failed")
		return nil
	}

	require.ErrorContains(t, l.Run(context.Background()), "disk full")
	require.Empty(t, errOut.String())
}
