package phase

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/stretchr/testify/require"
)

type mockconfigurer struct {
	linux.Ubuntu
	skew time.Duration
}

func (c *mockconfigurer) SystemTime(_ cfg.Host) (time.Time, error) {
	return time.Now().Add(c.skew), nil
}

type mockValidator struct {
	mockconfigurer
	err error
}

func (c *mockValidator) ValidateHost(_ cfg.Host) error {
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

func TestValidateUniqueHostname(t *testing.T) {
	makeHost := func(hostname string) *cluster.Host {
		h := &cluster.Host{Configurer: &mockconfigurer{}}
		h.Metadata.Hostname = hostname
		return h
	}

	t.Run("duplicate hostname same case", func(t *testing.T) {
		hosts := cluster.Hosts{makeHost("worker-1"), makeHost("worker-1")}
		p := &ValidateHosts{hncount: make(map[string]int, len(hosts))}
		for _, h := range hosts {
			p.hncount[h.KubernetesNodeName()]++
		}
		require.Error(t, p.validateUniqueHostname(context.Background(), hosts[0]))
	})

	t.Run("duplicate hostname different case", func(t *testing.T) {
		hosts := cluster.Hosts{makeHost("Worker-1"), makeHost("worker-1")}
		p := &ValidateHosts{hncount: make(map[string]int, len(hosts))}
		for _, h := range hosts {
			p.hncount[h.KubernetesNodeName()]++
		}
		require.Error(t, p.validateUniqueHostname(context.Background(), hosts[0]))
		require.Error(t, p.validateUniqueHostname(context.Background(), hosts[1]))
	})

	t.Run("unique hostnames", func(t *testing.T) {
		hosts := cluster.Hosts{makeHost("worker-1"), makeHost("worker-2")}
		p := &ValidateHosts{hncount: make(map[string]int, len(hosts))}
		for _, h := range hosts {
			p.hncount[h.KubernetesNodeName()]++
		}
		require.NoError(t, p.validateUniqueHostname(context.Background(), hosts[0]))
		require.NoError(t, p.validateUniqueHostname(context.Background(), hosts[1]))
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
