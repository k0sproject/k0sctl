package phase

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/rig/v2/rigtest"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

// newMockHost returns a host named hostname backed by a mock runner on which
// commands not set up by setup fail.
func newMockHost(t *testing.T, hostname string, setup func(mr *rigtest.MockRunner)) *cluster.Host {
	t.Helper()
	mr := rigtest.NewMockRunner()
	mr.ErrDefault = errors.New("no such file")
	mr.AddCommand(rigtest.Match("sudo.*true"), func(_ *rigtest.A) error { return nil })
	setup(mr)
	posixFS := remotefs.NewPosixFS(mr)
	client, err := rig.NewClient(
		rig.WithConnection(mr.MockConnection),
		rig.WithRemoteFSProvider(func(_ cmd.Runner) (remotefs.FS, error) {
			return posixFS, nil
		}),
	)
	require.NoError(t, err)
	return &cluster.Host{
		Client:     client,
		Configurer: &linux.Ubuntu{},
		Metadata:   cluster.HostMetadata{Hostname: hostname},
	}
}

// newUpgradedWorker returns a worker whose "k0s status" reports the given
// process ids in turn, with 0 standing for k0s not running. The last one is
// repeated once they run out. The node is always ready on v1.36.6.
func newUpgradedWorker(t *testing.T, pids ...int) *cluster.Host {
	t.Helper()
	return newMockHost(t, "worker0", func(mr *rigtest.MockRunner) {
		calls := 0
		mr.AddCommand(rigtest.Contains("status -o json"), func(a *rigtest.A) error {
			pid := pids[min(calls, len(pids)-1)]
			calls++
			if pid == 0 {
				return errors.New("dial unix /run/k0s/status.sock: connect: no such file or directory")
			}
			_, err := fmt.Fprintf(a.Stdout, `{"Version":"v1.36.6+k0s.0","Pid":%d,"Role":"worker"}`, pid)
			return err
		})
		mr.AddCommand(rigtest.Contains("kubectl"), func(a *rigtest.A) error {
			_, err := io.WriteString(a.Stdout, `{"status":{"conditions":[{"type":"Ready","status":"True"}],"nodeInfo":{"kubeletVersion":"v1.36.6+k0s"}}}`)
			return err
		})
	})
}

func fastRetry(t *testing.T) {
	t.Helper()
	interval, timeout := retry.Interval, retry.DefaultTimeout
	retry.Interval, retry.DefaultTimeout = time.Millisecond, time.Second
	t.Cleanup(func() { retry.Interval, retry.DefaultTimeout = interval, timeout })
}

func TestWaitUpgradedWorker_K0sKeepsRunning(t *testing.T) {
	fastRetry(t)
	h := newUpgradedWorker(t, 100)
	require.NoError(t, waitUpgradedWorker(t.Context(), h, version.MustParse("v1.36.6+k0s.0")))
}

func TestWaitUpgradedWorker_WaitsForK0sToStart(t *testing.T) {
	fastRetry(t)
	h := newUpgradedWorker(t, 0, 0, 100)
	require.NoError(t, waitUpgradedWorker(t.Context(), h, version.MustParse("v1.36.6+k0s.0")))
}

func TestWaitUpgradedWorker_WaitsForK0sRestartedAfterNodeReady(t *testing.T) {
	fastRetry(t)
	// k0s exits once the node is ready and comes back as a new process.
	h := newUpgradedWorker(t, 100, 0, 0, 200)
	require.NoError(t, waitUpgradedWorker(t.Context(), h, version.MustParse("v1.36.6+k0s.0")))
}

func TestWaitUpgradedWorker_FailsWhenK0sDoesNotComeBack(t *testing.T) {
	fastRetry(t)
	h := newUpgradedWorker(t, 100, 0)
	err := waitUpgradedWorker(t.Context(), h, version.MustParse("v1.36.6+k0s.0"))
	require.ErrorContains(t, err, "k0s is not running after the node became ready")
}

func TestWaitUpgradedWorker_FailsWhenK0sKeepsRestarting(t *testing.T) {
	fastRetry(t)
	h := newUpgradedWorker(t, 100, 200, 300, 400, 500)
	err := waitUpgradedWorker(t.Context(), h, version.MustParse("v1.36.6+k0s.0"))
	require.ErrorContains(t, err, "k0s keeps restarting")
}

func TestWaitUpgradedWorker_AllowsMaxK0sRestarts(t *testing.T) {
	fastRetry(t)
	pids := []int{100}
	for i := 1; i <= maxK0sRestarts; i++ {
		pids = append(pids, 100+i)
	}
	h := newUpgradedWorker(t, pids...)
	require.NoError(t, waitUpgradedWorker(t.Context(), h, version.MustParse("v1.36.6+k0s.0")))
}

func TestUpgradeWorkersCleanUp_KeepsUnhealthyWorkersCordoned(t *testing.T) {
	var mu sync.Mutex
	var uncordoned []string
	leader := newMockHost(t, "controller0", func(mr *rigtest.MockRunner) {
		mr.AddCommand(rigtest.Contains("uncordon"), func(a *rigtest.A) error {
			mu.Lock()
			defer mu.Unlock()
			uncordoned = append(uncordoned, a.Command)
			return nil
		})
	})
	healthy := newMockHost(t, "healthy", func(_ *rigtest.MockRunner) {})
	unhealthy := newMockHost(t, "unhealthy", func(_ *rigtest.MockRunner) {})

	p := &UpgradeWorkers{leader: leader, hosts: cluster.Hosts{healthy, unhealthy}}
	p.SetManager(&Manager{})
	p.keepCordoned.Store(unhealthy, struct{}{})
	p.CleanUp()

	require.Len(t, uncordoned, 1)
	require.Contains(t, uncordoned[0], "uncordon healthy")
}
