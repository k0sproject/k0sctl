package phase

import (
	"context"
	"fmt"
	"testing"
	"time"

	cfg "github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/os"
	"github.com/stretchr/testify/require"
)

type mockconfigurer struct {
	cfg.Linux
	linux.Ubuntu
	skew time.Duration
}

func (c *mockconfigurer) SystemTime(_ os.Host) (time.Time, error) {
	return time.Now().Add(c.skew), nil
}

type mockValidator struct {
	mockconfigurer
	err error
}

func (c *mockValidator) ValidateHost(os.Host) error {
	return c.err
}

func TestValidateClockSkew(t *testing.T) {
	hosts := []*cluster.Host{
		{
			Configurer: &mockconfigurer{skew: -10 * time.Second},
		},
		{
			Configurer: &mockconfigurer{skew: 10 * time.Second},
		},
		{
			Configurer: &mockconfigurer{skew: 1},
		},
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
		p.Config.Spec.Hosts[2].Configurer = &mockconfigurer{skew: time.Minute}
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
