package node

import (
	"context"
	"errors"
	"testing"

	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/retry"
	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/protocol"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/rig/v2/rigtest"
	"github.com/stretchr/testify/require"
)

// newTestHost returns a *cluster.Host backed by a mock runner. Commands that
// aren't explicitly matched fail, which also makes the KubeconfigPath lookup's
// `test -f` probe report the admin.conf as absent, so KubectlCmdf resolves to
// the kubelet.conf fallback path consistently across tests.
func newTestHost(t *testing.T, addOutput func(mr *rigtest.MockRunner)) *cluster.Host {
	t.Helper()
	mr := rigtest.NewMockRunner()
	mr.ErrDefault = errors.New("no such file")
	mr.AddCommand(rigtest.Match("sudo.*true"), func(a *rigtest.A) error { return nil })
	addOutput(mr)
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
		Metadata:   cluster.HostMetadata{Hostname: "node1"},
	}
}

func TestKubeNodeReadyFunc(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		h := newTestHost(t, func(mr *rigtest.MockRunner) {
			mr.AddCommandOutput(rigtest.Contains("kubectl"), `{"status":{"conditions":[{"type":"Ready","status":"True"}]}}`)
		})
		require.NoError(t, KubeNodeReadyFunc(h)(context.Background()))
	})

	t.Run("not ready", func(t *testing.T) {
		h := newTestHost(t, func(mr *rigtest.MockRunner) {
			mr.AddCommandOutput(rigtest.Contains("kubectl"), `{"status":{"conditions":[{"type":"Ready","status":"False"}]}}`)
		})
		err := KubeNodeReadyFunc(h)(context.Background())
		require.ErrorContains(t, err, "not ready")
	})

	t.Run("no ready condition", func(t *testing.T) {
		h := newTestHost(t, func(mr *rigtest.MockRunner) {
			mr.AddCommandOutput(rigtest.Contains("kubectl"), `{"status":{"conditions":[]}}`)
		})
		err := KubeNodeReadyFunc(h)(context.Background())
		require.ErrorContains(t, err, "'Ready' condition not found")
	})

	t.Run("invalid json", func(t *testing.T) {
		h := newTestHost(t, func(mr *rigtest.MockRunner) {
			mr.AddCommandOutput(rigtest.Contains("kubectl"), `not json`)
		})
		err := KubeNodeReadyFunc(h)(context.Background())
		require.ErrorContains(t, err, "failed to decode")
	})

	t.Run("non-retryable exec error", func(t *testing.T) {
		h := newTestHost(t, func(mr *rigtest.MockRunner) {
			mr.AddCommandFailure(rigtest.Contains("kubectl"), protocol.ErrNonRetryable)
		})
		err := KubeNodeReadyFunc(h)(context.Background())
		require.ErrorIs(t, err, retry.ErrAbort)
	})
}

func TestK0sDynamicConfigReadyFunc(t *testing.T) {
	t.Run("reconciled", func(t *testing.T) {
		h := newTestHost(t, func(mr *rigtest.MockRunner) {
			mr.AddCommandOutput(rigtest.Contains("kubectl"), `{"items":[{"reason":"SuccessfulReconcile"}]}`)
		})
		require.NoError(t, K0sDynamicConfigReadyFunc(h)(context.Background()))
	})

	t.Run("not yet reconciled", func(t *testing.T) {
		h := newTestHost(t, func(mr *rigtest.MockRunner) {
			mr.AddCommandOutput(rigtest.Contains("kubectl"), `{"items":[]}`)
		})
		err := K0sDynamicConfigReadyFunc(h)(context.Background())
		require.ErrorContains(t, err, "dynamic config not ready")
	})

	t.Run("exec error", func(t *testing.T) {
		h := newTestHost(t, func(_ *rigtest.MockRunner) {})
		err := K0sDynamicConfigReadyFunc(h)(context.Background())
		require.ErrorContains(t, err, "failed to get k0s config status events")
	})
}
