package phase

import (
	"context"
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
