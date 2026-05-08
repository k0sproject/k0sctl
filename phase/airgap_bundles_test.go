package phase

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	linuxcfg "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/airgap"
	v1beta1 "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

func TestAirgapBundlesPreparePlansWorkerCapableHosts(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")
	worker := airgapHost("worker", "amd64")
	controllerWorker := airgapHost("controller+worker", "arm64")
	controllerWorker.DataDir = "/opt/k0s"
	single := airgapHost("single", "riscv64")
	single.Reset = true
	controller := airgapHost("controller", "amd64")
	cfg := airgapConfig(k0sVersion, cluster.Hosts{worker, controllerWorker, single, controller})

	phase := &AirgapBundles{}
	require.NoError(t, phase.Prepare(cfg))

	require.True(t, phase.ShouldRun())
	require.Len(t, phase.plans, 2)
	require.Equal(t, worker, phase.plans[0].Host)
	require.Equal(t, "/var/lib/k0s/images/k0s-airgap-bundle-v1.34.1+k0s.0-amd64", phase.plans[0].Destination)
	require.Equal(t, controllerWorker, phase.plans[1].Host)
	require.Equal(t, "/opt/k0s/images/k0s-airgap-bundle-v1.34.1+k0s.0-arm64", phase.plans[1].Destination)
	require.Equal(t, 0, phase.planIndexes[worker])
	require.Equal(t, 1, phase.planIndexes[controllerWorker])
}

func TestAirgapBundlesDryRunOutput(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")
	cfg := airgapConfig(k0sVersion, cluster.Hosts{airgapHost("worker", "amd64")})
	var writer bytes.Buffer
	manager := Manager{Config: cfg, DryRun: true, Writer: &writer}
	manager.AddPhase(&AirgapBundles{})

	require.NoError(t, manager.Run(context.Background()))

	output := writer.String()
	require.Contains(t, output, "dry-run: cluster state altering actions would be performed:")
	require.Contains(t, output, "upload airgap bundle k0s-airgap-bundle-v1.34.1+k0s.0-amd64 (linux/amd64) => /var/lib/k0s/images/k0s-airgap-bundle-v1.34.1+k0s.0-amd64")
}

func TestAirgapBundlesRequiresVersion(t *testing.T) {
	cfg := airgapConfig(nil, cluster.Hosts{airgapHost("worker", "amd64")})

	phase := &AirgapBundles{}
	require.ErrorContains(t, phase.Prepare(cfg), "spec.k0s.version is required when airgap is enabled")
}

func TestAirgapBundlesLocalSourceUsesConfiguredSHA256(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")
	cfg := airgapConfig(k0sVersion, cluster.Hosts{airgapHost("worker", "amd64")})
	cfg.Spec.K0s.Airgap.Source = cluster.AirgapSourceLocal
	bundle := filepath.Join(t.TempDir(), "bundle")
	require.NoError(t, os.WriteFile(bundle, []byte("bundle"), 0o644))
	cfg.Spec.K0s.Airgap.Path = bundle
	cfg.Spec.K0s.Airgap.SHA256 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	phase := &AirgapBundles{}
	require.NoError(t, phase.Prepare(cfg))
	require.Len(t, phase.plans, 1)
	require.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", phase.plans[0].Artifact.SHA256)
}

func TestAirgapBundlesLocalSourceFileRejectsMixedBundles(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")
	bundle := filepath.Join(t.TempDir(), "bundle")
	require.NoError(t, os.WriteFile(bundle, []byte("bundle"), 0o644))
	cfg := airgapConfig(k0sVersion, cluster.Hosts{
		airgapHost("worker", "amd64"),
		airgapHost("worker", "arm64"),
	})
	cfg.Spec.K0s.Airgap.Source = cluster.AirgapSourceLocal
	cfg.Spec.K0s.Airgap.Path = bundle

	phase := &AirgapBundles{}
	require.ErrorContains(t, phase.Prepare(cfg), "spec.k0s.airgap.path points to a single file but planned hosts require multiple airgap bundles")
}

func TestAirgapBundlesPopulateCachesDeduplicatesDownloads(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")
	oldCacheHome, hadCacheHome := os.LookupEnv("XDG_CACHE_HOME")
	require.NoError(t, os.Setenv("XDG_CACHE_HOME", t.TempDir()))
	xdg.Reload()
	t.Cleanup(func() {
		if hadCacheHome {
			require.NoError(t, os.Setenv("XDG_CACHE_HOME", oldCacheHome))
		} else {
			require.NoError(t, os.Unsetenv("XDG_CACHE_HOME"))
		}
		xdg.Reload()
	})

	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, err := fmt.Fprint(w, "bundle")
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	artifact := airgap.Artifact{
		Name: "k0s-airgap-bundle-v1.34.1+k0s.0-amd64",
		URL:  server.URL + "/bundle",
		OS:   "linux",
		Arch: "amd64",
	}
	phase := &AirgapBundles{
		GenericPhase: GenericPhase{Config: airgapConfig(k0sVersion, nil)},
		plans: []airgap.Plan{
			{Host: airgapHost("worker", "amd64"), Artifact: artifact},
			{Host: airgapHost("worker", "amd64"), Artifact: artifact},
		},
	}

	require.NoError(t, phase.populateCaches(context.Background()))
	require.Equal(t, 1, requests)
	require.NotEmpty(t, phase.plans[0].LocalPath)
	require.Equal(t, phase.plans[0].LocalPath, phase.plans[1].LocalPath)
}

func airgapConfig(k0sVersion *version.Version, hosts cluster.Hosts) *v1beta1.Cluster {
	return &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: hosts,
			K0s: &cluster.K0s{
				Version: k0sVersion,
				Airgap: &cluster.Airgap{
					Enabled: true,
					Source:  cluster.AirgapSourceAuto,
					Mode:    cluster.AirgapModeUpload,
				},
			},
		},
	}
}

func airgapHost(role, arch string) *cluster.Host {
	return &cluster.Host{
		Role:       role,
		Configurer: &linuxcfg.Ubuntu{},
		Metadata: cluster.HostMetadata{
			Arch: arch,
		},
	}
}
