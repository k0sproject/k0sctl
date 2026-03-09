package phase

import (
	"strings"
	"testing"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
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

func TestConfigForSetsAPIAddressWhenIPv6NodeLocalLBEnabled(t *testing.T) {
	base := dig.Mapping{
		"spec": dig.Mapping{
			"api": dig.Mapping{},
			"network": dig.Mapping{
				"dualStack":              dig.Mapping{"primaryAddressFamily": "IPv6"},
				"nodeLocalLoadBalancing": dig.Mapping{"enabled": true},
			},
		},
	}

	clusterConfig := &v1beta1.Cluster{Spec: &cluster.Spec{K0s: &cluster.K0s{}}}
	p := &ConfigureK0s{GenericPhase: GenericPhase{Config: clusterConfig}, newBaseConfig: base}
	h := &cluster.Host{PrivateAddress: "fc00::101"}

	config, err := p.configFor(h)
	require.NoError(t, err)
	require.Equal(t, "fc00::101", apiAddressFromConfig(t, config))
}

func TestConfigForLeavesAPIAddressWhenIPv6NodeLocalLBDisabled(t *testing.T) {
	base := dig.Mapping{
		"spec": dig.Mapping{
			"api": dig.Mapping{},
			"network": dig.Mapping{
				"dualStack":              dig.Mapping{"primaryAddressFamily": "IPv6"},
				"nodeLocalLoadBalancing": dig.Mapping{"enabled": false},
			},
		},
	}

	clusterConfig := &v1beta1.Cluster{Spec: &cluster.Spec{K0s: &cluster.K0s{}}}
	p := &ConfigureK0s{GenericPhase: GenericPhase{Config: clusterConfig}, newBaseConfig: base}
	h := &cluster.Host{PrivateAddress: "fc00::102"}

	config, err := p.configFor(h)
	require.NoError(t, err)
	require.Equal(t, "", apiAddressFromConfig(t, config))
}

type apiSpec struct {
	Spec struct {
		API struct {
			Address string `yaml:"address"`
		} `yaml:"api"`
	} `yaml:"spec"`
}

func apiAddressFromConfig(t *testing.T, cfg string) string {
	t.Helper()
	parts := strings.SplitN(cfg, "\n", 2)
	require.Len(t, parts, 2)
	var parsed apiSpec
	require.NoError(t, yaml.Unmarshal([]byte(parts[1]), &parsed))
	return parsed.Spec.API.Address
}
