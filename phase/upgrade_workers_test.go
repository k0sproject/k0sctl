package phase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creasty/defaults"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
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
	// The test's own commands go first, so that the sudo probe matcher below
	// can't take over a command that happens to contain "true".
	setup(mr)
	mr.AddCommand(rigtest.Match("sudo.*true"), func(_ *rigtest.A) error { return nil })
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

func TestWaitUpgradedWorker_CountsEachRestartWhileNodeNotReady(t *testing.T) {
	fastRetry(t)
	// k0s restarts between each node status query until the node is ready.
	// Checking k0s only after the node is ready would see one restart.
	const notReady = maxK0sRestarts + 1
	var mu sync.Mutex
	nodeQueries := 0
	h := newMockHost(t, "worker0", func(mr *rigtest.MockRunner) {
		mr.AddCommand(rigtest.Contains("status -o json"), func(a *rigtest.A) error {
			mu.Lock()
			pid := 100 + 100*min(nodeQueries, notReady)
			mu.Unlock()
			_, err := fmt.Fprintf(a.Stdout, `{"Version":"v1.36.6+k0s.0","Pid":%d,"Role":"worker"}`, pid)
			return err
		})
		mr.AddCommand(rigtest.Contains("kubectl"), func(a *rigtest.A) error {
			mu.Lock()
			ready := "True"
			if nodeQueries < notReady {
				ready = "False"
			}
			nodeQueries++
			mu.Unlock()
			_, err := fmt.Fprintf(a.Stdout, `{"status":{"conditions":[{"type":"Ready","status":"%s"}],"nodeInfo":{"kubeletVersion":"v1.36.6+k0s"}}}`, ready)
			return err
		})
	})
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
	p.Config = &v1beta1.Cluster{Spec: &cluster.Spec{}}
	p.keepCordoned.Store(unhealthy, struct{}{})
	p.CleanUp()

	require.Len(t, uncordoned, 1)
	require.Contains(t, uncordoned[0], "uncordon healthy")
}

func TestUpgradeWorker_KeepsCordonedWhenStoppedStateWaitFails(t *testing.T) {
	fastRetry(t)
	h := newMockHost(t, "worker0", func(mr *rigtest.MockRunner) {
		mr.AddCommand(rigtest.Contains("stat /run/systemd/system"), func(_ *rigtest.A) error { return nil })
		mr.AddCommand(rigtest.Contains("systemctl stop"), func(_ *rigtest.A) error { return nil })
		// The service keeps reporting as running after the stop.
		mr.AddCommand(rigtest.Contains("systemctl status"), func(_ *rigtest.A) error { return nil })
	})
	p := &UpgradeWorkers{}
	p.SetManager(&Manager{})

	// The stop waits for the stopped state until the context expires.
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	require.Error(t, p.upgradeWorker(ctx, h))
	_, kept := p.keepCordoned.Load(h)
	require.True(t, kept, "the worker is not kept cordoned")
}

func TestUpgradeWorker_ServiceLookupFailureDoesNotKeepCordoned(t *testing.T) {
	// No init system is detected, so the upgrade fails before k0s is stopped.
	h := newMockHost(t, "worker0", func(_ *rigtest.MockRunner) {})
	p := &UpgradeWorkers{}
	p.SetManager(&Manager{})

	require.ErrorContains(t, p.upgradeWorker(t.Context(), h), "get service")
	_, kept := p.keepCordoned.Load(h)
	require.False(t, kept, "the worker is kept cordoned")
}

func TestUpgradeWorkersStep_SkipsRemainingStepsAfterFailure(t *testing.T) {
	failing := newMockHost(t, "failing", func(_ *rigtest.MockRunner) {})
	healthy := newMockHost(t, "healthy", func(_ *rigtest.MockRunner) {})
	p := &UpgradeWorkers{}

	var mu sync.Mutex
	ran := map[string][]string{}
	record := func(name string, fail bool) func(context.Context, *cluster.Host) error {
		return func(_ context.Context, h *cluster.Host) error {
			mu.Lock()
			ran[h.Metadata.Hostname] = append(ran[h.Metadata.Hostname], name)
			mu.Unlock()
			if fail && h == failing {
				return errors.New("k0s is not running after the node became ready")
			}
			return nil
		}
	}

	err := cluster.Hosts{failing, healthy}.ParallelEach(t.Context(),
		p.step(record("drain", false)),
		p.step(record("upgrade", true)),
		p.step(record("uncordon", false)),
		p.step(record("finish", false)),
	)
	require.ErrorContains(t, err, "k0s is not running")
	require.Equal(t, []string{"drain", "upgrade"}, ran["failing"])
	require.Equal(t, []string{"drain", "upgrade", "uncordon", "finish"}, ran["healthy"])
}

func TestUpgradeWorkersRun_FailedDrainSkipsUpgradeAndUncordon(t *testing.T) {
	var mu sync.Mutex
	var commands []string
	leader := newMockHost(t, "controller0", func(mr *rigtest.MockRunner) {
		mr.AddCommand(rigtest.Contains("kubectl"), func(a *rigtest.A) error {
			mu.Lock()
			commands = append(commands, a.Command)
			mu.Unlock()
			if strings.Contains(a.Command, "drain") {
				return errors.New("cannot evict pod as it would violate the pod's disruption budget")
			}
			return nil
		})
	})
	worker := newMockHost(t, "worker0", func(_ *rigtest.MockRunner) {})

	p := &UpgradeWorkers{leader: leader, hosts: cluster.Hosts{worker}}
	p.SetManager(&Manager{})
	p.Config = &v1beta1.Cluster{Spec: &cluster.Spec{Options: cluster.Options{
		Concurrency: cluster.ConcurrencyOption{Limit: 30, WorkerDisruptionPercent: 10},
	}}}

	err := p.Run(t.Context())
	require.ErrorContains(t, err, "drain node")
	// Reaching the upgrade step would have failed on the service lookup with another error.
	require.NotContains(t, err.Error(), "get service")

	var uncordons int
	for _, c := range commands {
		if strings.Contains(c, "uncordon") {
			uncordons++
		}
	}
	require.Zero(t, uncordons, "the worker was uncordoned during Run: %v", commands)
}

func TestUpgradeWorkers_FailedDrainWithEvictTaintIsRestoredByCleanUp(t *testing.T) {
	const evictTaint = "k0sctl.k0sproject.io/evict=true:NoExecute"
	var mu sync.Mutex
	var commands []string
	leader := newMockHost(t, "controller0", func(mr *rigtest.MockRunner) {
		mr.AddCommand(rigtest.Contains("kubectl"), func(a *rigtest.A) error {
			mu.Lock()
			commands = append(commands, a.Command)
			mu.Unlock()
			switch {
			case strings.Contains(a.Command, " drain "):
				return errors.New("cannot evict pod as it would violate the pod's disruption budget")
			case strings.Contains(a.Command, "get node"):
				_, err := io.WriteString(a.Stdout, evictTaint+"\n")
				return err
			}
			return nil
		})
	})
	drained := newMockHost(t, "drained", func(_ *rigtest.MockRunner) {})
	unhealthy := newMockHost(t, "unhealthy", func(_ *rigtest.MockRunner) {})

	opts := cluster.Options{Concurrency: cluster.ConcurrencyOption{Limit: 30, WorkerDisruptionPercent: 10}}
	require.NoError(t, defaults.Set(&opts.EvictTaint))
	opts.EvictTaint.Enabled = true
	p := &UpgradeWorkers{leader: leader, hosts: cluster.Hosts{drained}}
	p.SetManager(&Manager{})
	p.Config = &v1beta1.Cluster{Spec: &cluster.Spec{Options: opts}}

	require.ErrorContains(t, p.Run(t.Context()), "drain node")
	require.True(t, slices.ContainsFunc(commands, func(c string) bool {
		return strings.Contains(c, "taint nodes --overwrite drained") && strings.Contains(c, evictTaint)
	}), "the eviction taint was not added: %v", commands)

	// A worker that did not come back healthy keeps its cordon and eviction taint.
	p.hosts = append(p.hosts, unhealthy)
	p.keepCordoned.Store(unhealthy, struct{}{})
	commands = nil
	p.CleanUp()

	require.True(t, slices.ContainsFunc(commands, func(c string) bool { return strings.Contains(c, "uncordon drained") }), "drained worker was not uncordoned: %v", commands)
	require.True(t, slices.ContainsFunc(commands, func(c string) bool {
		return strings.Contains(c, "taint nodes drained") && strings.Contains(c, evictTaint+"-")
	}), "the eviction taint was not removed from the drained worker: %v", commands)
	require.False(t, slices.ContainsFunc(commands, func(c string) bool { return strings.Contains(c, "unhealthy") }), "the unhealthy worker was touched: %v", commands)
}
