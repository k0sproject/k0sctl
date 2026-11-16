package phase

import (
	"context"
	"errors"
	"testing"

	linuxcfg "github.com/k0sproject/k0sctl/configurer/linux"
	v1beta1 "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStageBinariesPrepareFiltersHosts(t *testing.T) {
	targetVersion := version.MustParse("v1.29.0+k0s.0")

	eligible := &cluster.Host{}
	eligibleProvider := &fakeBinaryProvider{needsUpgrade: true}
	eligible.SetK0sBinaryProvider(eligibleProvider)

	resetHost := &cluster.Host{Reset: true}
	resetHost.SetK0sBinaryProvider(&fakeBinaryProvider{needsUpgrade: true})

	upToDate := &cluster.Host{}
	upToDate.SetK0sBinaryProvider(&fakeBinaryProvider{})

	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{eligible, resetHost, upToDate},
			K0s:   &cluster.K0s{Version: targetVersion},
		},
	}

	phase := &StageBinaries{}
	require.NoError(t, phase.Prepare(cfg))

	require.Len(t, phase.hosts, 1)
	assert.Same(t, eligible, phase.hosts[0])
}

func TestStageBinariesRunStagesBinary(t *testing.T) {
	targetVersion := version.MustParse("v1.30.0+k0s.0")

	host := &cluster.Host{}
	provider := &fakeBinaryProvider{needsUpgrade: true, stagePath: "/tmp/k0s-stage"}
	host.SetK0sBinaryProvider(provider)

	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{host},
			K0s:   &cluster.K0s{Version: targetVersion},
		},
	}

	phase := &StageBinaries{}
	require.NoError(t, phase.Prepare(cfg))
	phase.SetManager(&Manager{})

	require.NoError(t, phase.Run(context.Background()))
	assert.Equal(t, provider.stagePath, host.Metadata.K0sBinaryTempFile)
	assert.Equal(t, 1, provider.stageCalls)
}

func TestStageBinariesPopulateCacheBeforeStaging(t *testing.T) {
	targetVersion := version.MustParse("v1.30.1+k0s.0")

	host := &cluster.Host{Configurer: &linuxcfg.Ubuntu{}, Metadata: cluster.HostMetadata{Arch: "amd64"}}
	provider := &fakeCachingBinaryProvider{
		fakeBinaryProvider: &fakeBinaryProvider{needsUpgrade: true, stagePath: "/tmp/k0s-stage", isUpload: true},
		cacheKey:           "amd64/linux/v1.30.1+k0s.0",
	}
	host.SetK0sBinaryProvider(provider)

	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{host},
			K0s:   &cluster.K0s{Version: targetVersion},
		},
	}

	phase := &StageBinaries{}
	require.NoError(t, phase.Prepare(cfg))
	phase.SetManager(&Manager{})

	require.NoError(t, phase.Run(context.Background()))
	assert.Equal(t, 1, provider.cacheCalls)
}

func TestStageBinariesDeduplicatesCacheByKey(t *testing.T) {
	targetVersion := version.MustParse("v1.30.1+k0s.0")

	// Two hosts with the same cache key (same version/os/arch) — cache should be populated once.
	makeHost := func() *cluster.Host {
		h := &cluster.Host{Configurer: &linuxcfg.Ubuntu{}, Metadata: cluster.HostMetadata{Arch: "amd64"}}
		h.SetK0sBinaryProvider(&fakeCachingBinaryProvider{
			fakeBinaryProvider: &fakeBinaryProvider{needsUpgrade: true, stagePath: "/tmp/k0s-stage", isUpload: true},
			cacheKey:           "amd64/linux/v1.30.1+k0s.0",
		})
		return h
	}
	host1, host2 := makeHost(), makeHost()

	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{host1, host2},
			K0s:   &cluster.K0s{Version: targetVersion},
		},
	}

	phase := &StageBinaries{}
	require.NoError(t, phase.Prepare(cfg))
	phase.SetManager(&Manager{})

	require.NoError(t, phase.Run(context.Background()))
	prov1, err := host1.K0sBinaryProvider(targetVersion)
	require.NoError(t, err)
	p1 := prov1.(*fakeCachingBinaryProvider)
	prov2, err := host2.K0sBinaryProvider(targetVersion)
	require.NoError(t, err)
	p2 := prov2.(*fakeCachingBinaryProvider)
	assert.Equal(t, 1, p1.cacheCalls+p2.cacheCalls, "expected exactly one EnsureCached call across both hosts")
}

func TestStageBinariesPropagatesCacheErrors(t *testing.T) {
	targetVersion := version.MustParse("v1.30.2+k0s.0")

	host := &cluster.Host{Configurer: &linuxcfg.Ubuntu{}, Metadata: cluster.HostMetadata{Arch: "arm64"}}
	provider := &fakeCachingBinaryProvider{
		fakeBinaryProvider: &fakeBinaryProvider{needsUpgrade: true, stagePath: "/tmp/k0s-stage", isUpload: true},
		cacheKey:           "arm64/linux/v1.30.2+k0s.0",
		cacheErr:           errors.New("cache failed"),
	}
	host.SetK0sBinaryProvider(provider)

	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{host},
			K0s:   &cluster.K0s{Version: targetVersion},
		},
	}

	phase := &StageBinaries{}
	require.NoError(t, phase.Prepare(cfg))
	phase.SetManager(&Manager{})

	err := phase.Run(context.Background())
	require.Error(t, err)
	assert.Equal(t, 1, provider.cacheCalls)
	assert.Equal(t, 0, provider.stageCalls)
}

func TestStageBinariesCleanUpUsesProviders(t *testing.T) {
	targetVersion := version.MustParse("v1.31.0+k0s.0")

	host := &cluster.Host{}
	provider := &fakeBinaryProvider{needsUpgrade: true, stagePath: "/tmp/k0s-stage"}
	host.SetK0sBinaryProvider(provider)

	cfg := &v1beta1.Cluster{
		Spec: &cluster.Spec{
			Hosts: cluster.Hosts{host},
			K0s:   &cluster.K0s{Version: targetVersion},
		},
	}

	phase := &StageBinaries{}
	require.NoError(t, phase.Prepare(cfg))
	phase.SetManager(&Manager{})

	require.NoError(t, phase.Run(context.Background()))
	phase.CleanUp()

	assert.Equal(t, 1, provider.cleanupCalls)
}

// fakeBinaryProvider is a test double for k0s.BinaryProvider.
// The provider is the sole authority for NeedsUpgrade — set needsUpgrade: true
// on hosts that should be staged. Host.Metadata.NeedsUpgrade is not consulted
// by StageBinaries.Prepare and must not be used as a substitute here.
type fakeBinaryProvider struct {
	needsUpgrade bool
	isUpload     bool
	stagePath    string
	stageErr     error
	stageCalls   int
	cleanupCalls int
}

func (p *fakeBinaryProvider) NeedsUpgrade() bool { return p.needsUpgrade }
func (p *fakeBinaryProvider) IsUpload() bool      { return p.isUpload }

func (p *fakeBinaryProvider) Stage(_ context.Context) (string, error) {
	p.stageCalls++
	if p.stageErr != nil {
		return "", p.stageErr
	}
	return p.stagePath, nil
}

func (p *fakeBinaryProvider) CleanUp(context.Context) {
	p.cleanupCalls++
}

type fakeCachingBinaryProvider struct {
	*fakeBinaryProvider
	cacheKey   string
	cacheCalls int
	cacheErr   error
}

func (p *fakeCachingBinaryProvider) BinaryCacheKey() (string, error) { return p.cacheKey, nil }
func (p *fakeCachingBinaryProvider) EnsureCached(_ context.Context) error {
	p.cacheCalls++
	return p.cacheErr
}
