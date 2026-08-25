package phase

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/k0sproject/dig"
	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/protocol/ssh"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func testController(address, privateAddress string) *cluster.Host {
	return &cluster.Host{
		Role:            "controller",
		PrivateAddress:  privateAddress,
		CompositeConfig: rig.CompositeConfig{SSH: &ssh.Config{Address: address}},
	}
}

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

	config, err := p.configFor(h, base.Dup())
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

	config, err := p.configFor(h, base.Dup())
	require.NoError(t, err)
	require.Empty(t, apiAddressFromConfig(t, config))
}

func TestControllerSansAppendsOwnAddresses(t *testing.T) {
	base := []string{"lb.example.com"}

	sans := hostsSans(base, testController("10.0.0.1", "172.16.0.1"))
	require.Equal(t, []string{"lb.example.com", "10.0.0.1", "172.16.0.1"}, sans)

	sans = hostsSans(base, testController("10.0.0.2", ""))
	require.Equal(t, []string{"lb.example.com", "10.0.0.2"}, sans)
}

func TestControllerSansDoesNotMutateBase(t *testing.T) {
	// build base via append so the slice has spare capacity, which would
	// expose backing array sharing between the per-controller copies
	var base []string
	base = append(base, "lb.example.com")

	sansA := hostsSans(base, testController("10.0.0.1", "172.16.0.1"))
	sansB := hostsSans(base, testController("10.0.0.2", "172.16.0.2"))

	require.Equal(t, []string{"lb.example.com"}, base)
	require.Equal(t, []string{"lb.example.com", "10.0.0.1", "172.16.0.1"}, sansA)
	require.Equal(t, []string{"lb.example.com", "10.0.0.2", "172.16.0.2"}, sansB)
}

func TestControllerSansDedup(t *testing.T) {
	base := []string{"10.0.0.1", "172.16.0.1"}
	sans := hostsSans(base, testController("10.0.0.1", "172.16.0.1"))
	require.Equal(t, base, sans)

	sans = hostsSans(nil, testController("10.0.0.1", "10.0.0.1"))
	require.Equal(t, []string{"10.0.0.1"}, sans)
}

func TestConfigForUsesPerHostSans(t *testing.T) {
	newBaseConfig := dig.Mapping{
		"spec": dig.Mapping{
			"api": dig.Mapping{
				"sans": []string{"lb.example.com"},
			},
		},
	}

	clusterConfig := &v1beta1.Cluster{Spec: &cluster.Spec{K0s: &cluster.K0s{}}}
	p := &ConfigureK0s{GenericPhase: GenericPhase{Config: clusterConfig}, newBaseConfig: newBaseConfig}

	hosts := []*cluster.Host{
		testController("10.0.0.1", "172.16.0.1"),
		testController("10.0.0.2", "172.16.0.2"),
	}

	base := []string{"lb.example.com"}
	for i, h := range hosts {
		hostBaseConfig := p.newBaseConfig.Dup()
		hostBaseConfig.DigMapping("spec", "api")["sans"] = hostsSans(base, h)

		config, err := p.configFor(h, hostBaseConfig)
		require.NoError(t, err)

		expected := []string{"lb.example.com", h.Address(), h.PrivateAddress}
		require.Equal(t, expected, sansFromConfig(t, config), "host %d", i+1)
	}

	// shared base config must remain untouched
	require.Equal(t, []string{"lb.example.com"}, newBaseConfig.Dig("spec", "api", "sans"))
}

type apiSpec struct {
	Spec struct {
		API struct {
			Address string   `yaml:"address"`
			Sans    []string `yaml:"sans"`
		} `yaml:"api"`
	} `yaml:"spec"`
}

func parseAPISpec(t *testing.T, cfg string) apiSpec {
	t.Helper()
	parts := strings.SplitN(cfg, "\n", 2)
	require.Len(t, parts, 2)
	var parsed apiSpec
	require.NoError(t, yaml.Unmarshal([]byte(parts[1]), &parsed))
	return parsed
}

func apiAddressFromConfig(t *testing.T, cfg string) string {
	t.Helper()
	return parseAPISpec(t, cfg).Spec.API.Address
}

func sansFromConfig(t *testing.T, cfg string) []string {
	t.Helper()
	return parseAPISpec(t, cfg).Spec.API.Sans
}

func TestControllersNeedingRestart(t *testing.T) {
	running := &cluster.Host{Role: "controller"}
	running.Metadata.K0sRunningVersion = version.MustParse("v1.36.3+k0s.2")

	notRunning := &cluster.Host{Role: "controller"}

	pendingUpgrade := &cluster.Host{Role: "controller"}
	pendingUpgrade.Metadata.K0sRunningVersion = version.MustParse("v1.36.2+k0s.0")
	pendingUpgrade.Metadata.NeedsUpgrade = true

	got := controllersNeedingRestart(cluster.Hosts{running, notRunning, pendingUpgrade})

	// A host that is not running k0s yet is started by a later phase, and a host
	// pending an upgrade is restarted by UpgradeControllers.
	require.Equal(t, cluster.Hosts{running}, got)
}

// runningController returns a controller that is up and not pending an upgrade,
// which is the only kind that configureControllers restarts.
func runningController(t *testing.T, address string) *cluster.Host {
	t.Helper()
	h := &cluster.Host{Role: "controller"}
	h.SSH = &ssh.Config{Address: address}
	h.Metadata.K0sRunningVersion = version.MustParse("v1.36.3+k0s.2")
	return h
}

func TestConfigureControllersRestartsOneAtATime(t *testing.T) {
	h1 := runningController(t, "10.0.0.1")
	h2 := runningController(t, "10.0.0.2")

	var (
		inFlight    atomic.Int32
		maxInFlight atomic.Int32
	)
	started := make(chan *cluster.Host, 2)
	release := make(chan struct{})

	restart := func(_ context.Context, h *cluster.Host) error {
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		defer inFlight.Add(-1)

		started <- h
		if h == h1 {
			// Stand in for a controller that has restarted but is not serving
			// /readyz yet.
			<-release
		}
		return nil
	}

	p := &ConfigureK0s{}
	p.SetManager(&Manager{})

	done := make(chan error, 1)
	go func() {
		done <- p.configureControllers(t.Context(), cluster.Hosts{h1, h2}, func(context.Context, *cluster.Host) error { return nil }, restart)
	}()

	require.Same(t, h1, <-started, "the first controller should be restarted first")

	select {
	case h := <-started:
		close(release)
		t.Fatalf("%s was restarted while the first controller was still coming up", h)
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	require.Same(t, h2, <-started, "the second controller should be restarted after the first one is ready")
	require.NoError(t, <-done)

	// The real guard: an accidental switch back to parallel execution would let
	// the second restart enter while the first one is held.
	require.Equal(t, int32(1), maxInFlight.Load(), "controllers were restarted concurrently")
}

func TestConfigureControllersRestartsHostsThatInstalledWhenAnotherFails(t *testing.T) {
	h1 := runningController(t, "10.0.0.1")
	h2 := runningController(t, "10.0.0.2")

	install := func(_ context.Context, h *cluster.Host) error {
		if h == h2 {
			return errors.New("disk full")
		}
		return nil
	}

	var restarted cluster.Hosts
	restart := func(_ context.Context, h *cluster.Host) error {
		restarted = append(restarted, h)
		return nil
	}

	p := &ConfigureK0s{}
	p.SetManager(&Manager{})

	err := p.configureControllers(t.Context(), cluster.Hosts{h1, h2}, install, restart)
	require.ErrorContains(t, err, "disk full")

	// h1 has the new configuration on disk. If it is not restarted now, the next
	// apply reads that file back as the existing config and skips the host, so
	// it would keep running the old configuration indefinitely.
	require.Equal(t, cluster.Hosts{h1}, restarted)
}

func TestConfigureControllersNamesTheHostThatFailedToRestart(t *testing.T) {
	h := runningController(t, "10.0.0.1")

	restart := func(context.Context, *cluster.Host) error {
		return errors.New("wait for k0s service start")
	}

	p := &ConfigureK0s{}
	p.SetManager(&Manager{})

	err := p.configureControllers(t.Context(), cluster.Hosts{h}, func(context.Context, *cluster.Host) error { return nil }, restart)
	require.ErrorContains(t, err, "wait for k0s service start")
	require.ErrorContains(t, err, "10.0.0.1", "the error should identify the controller that failed")
}
