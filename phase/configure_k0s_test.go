package phase

import (
	"testing"

	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

func TestBuildConfigValidateCommandAddsFeatureGates(t *testing.T) {
	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			K0s: &cluster.K0s{Version: version.MustParse("v1.24.0+k0s.0")},
		},
	}

	p := &ConfigureK0s{GenericPhase: GenericPhase{Config: cfg}}
	h := &cluster.Host{Configurer: &linux.Debian{}}
	h.InstallFlags.Add("--feature-gates=IPv6DualStack=true")

	cmd := p.buildConfigValidateCommand(h, "/etc/k0s/config.yaml")

	require.Contains(t, cmd, "config validate --config=\"/etc/k0s/config.yaml\"")
	require.Contains(t, cmd, "--feature-gates=IPv6DualStack=true")
}
