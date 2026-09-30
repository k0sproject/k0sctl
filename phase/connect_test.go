package phase

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/rigtest"
	"github.com/stretchr/testify/require"
)

var (
	errRejected    = fmt.Errorf("%w: ssh dial: ssh: unable to authenticate", rig.ErrAuthFailed)
	errUnreachable = errors.New("ssh dial: connect: connection refused")
)

// scriptedConnection returns the scripted errors from Connect in order and then
// keeps returning the last one.
type scriptedConnection struct {
	*rigtest.MockConnection

	mu     sync.Mutex
	script []error
	calls  int
}

func (s *scriptedConnection) Connect(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.script[min(s.calls, len(s.script)-1)]
	s.calls++
	return err
}

func (s *scriptedConnection) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func scriptedHost(t *testing.T, script ...error) (*cluster.Host, *scriptedConnection) {
	t.Helper()
	conn := &scriptedConnection{MockConnection: rigtest.NewMockConnection(), script: script}
	client, err := rig.NewClient(rig.WithConnection(conn), rig.WithRetry(false))
	require.NoError(t, err)
	return &cluster.Host{Client: client}, conn
}

func runConnect(t *testing.T, h *cluster.Host) error {
	t.Helper()
	interval := retry.Interval
	retry.Interval = time.Millisecond
	t.Cleanup(func() { retry.Interval = interval })

	p := &Connect{GenericPhase: GenericPhase{
		Config:  &v1beta1.Cluster{Spec: &cluster.Spec{Hosts: cluster.Hosts{h}}},
		manager: &Manager{},
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return p.Run(ctx)
}

func TestConnect_StopsAfterRepeatedAuthRejections(t *testing.T) {
	h, conn := scriptedHost(t, errRejected)

	// ParallelEach flattens per-host errors into text, so assert on the message.
	err := runConnect(t, h)
	require.ErrorContains(t, err, fmt.Sprintf("credentials rejected %d times in a row", authRejectionLimit))
	require.ErrorContains(t, err, "unable to authenticate")
	require.Equal(t, authRejectionLimit, conn.Calls())
}

func TestConnect_OtherErrorsResetAuthRejectionCount(t *testing.T) {
	script := make([]error, 0, 2*authRejectionLimit)
	for range authRejectionLimit - 1 {
		script = append(script, errRejected)
	}
	script = append(script, errUnreachable)
	for range authRejectionLimit - 1 {
		script = append(script, errRejected)
	}
	script = append(script, nil)
	h, conn := scriptedHost(t, script...)

	require.NoError(t, runConnect(t, h))
	require.Equal(t, len(script), conn.Calls())
}
