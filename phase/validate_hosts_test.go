package phase

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	cfg "github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/rig/v2/rigtest"
	"github.com/stretchr/testify/require"
)

type mockconfigurer struct {
	cfg.Linux
	linux.Ubuntu
}

type mockValidator struct {
	mockconfigurer
	err error
}

func (c *mockValidator) ValidateHost(_ cfg.Host) error {
	return c.err
}

// skewHost returns a *cluster.Host whose FS().SystemTime() reports a remote
// clock offset from local time by the given skew. SystemTime resolves time by
// running `date -u +%s`, so the mock runner answers that command with the
// adjusted unix timestamp.
func skewHost(t *testing.T, skew time.Duration) *cluster.Host {
	t.Helper()
	mr := rigtest.NewMockRunner()
	mr.AddCommandOutput(rigtest.Contains("date -u +%s"), strconv.FormatInt(time.Now().Add(skew).Unix(), 10))
	posixFS := remotefs.NewPosixFS(mr)
	client, err := rig.NewClient(
		rig.WithConnection(mr.MockConnection),
		rig.WithRemoteFSProvider(func(_ cmd.Runner) (remotefs.FS, error) {
			return posixFS, nil
		}),
	)
	require.NoError(t, err)
	return &cluster.Host{Client: client}
}

func TestValidateClockSkew(t *testing.T) {
	hosts := []*cluster.Host{
		skewHost(t, -10*time.Second),
		skewHost(t, 10*time.Second),
		skewHost(t, 1),
	}

	config := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: hosts,
		},
	}

	p := &ValidateHosts{
		GenericPhase: GenericPhase{
			Config:  config,
			manager: &Manager{},
		},
	}

	t.Run("clock skew success", func(t *testing.T) {
		require.NoError(t, p.validateClockSkew(context.Background()))
	})
	t.Run("clock skew failure", func(t *testing.T) {
		p.Config.Spec.Hosts[2] = skewHost(t, time.Minute)
		require.Error(t, p.validateClockSkew(context.Background()))
	})
}

func TestValidateConfigurer(t *testing.T) {
	p := &ValidateHosts{}

	t.Run("non validating configurer", func(t *testing.T) {
		h := &cluster.Host{Configurer: &mockconfigurer{}}
		require.NoError(t, p.validateConfigurer(context.Background(), h))
	})

	t.Run("validating configurer", func(t *testing.T) {
		h := &cluster.Host{Configurer: &mockValidator{err: fmt.Errorf("missing feature")}}
		err := p.validateConfigurer(context.Background(), h)
		require.Error(t, err)
		require.Contains(t, err.Error(), "missing feature")
	})
}
