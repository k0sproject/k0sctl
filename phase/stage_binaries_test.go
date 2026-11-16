package phase

import (
	"context"
	"errors"
	"testing"

	linuxcfg "github.com/k0sproject/k0sctl/configurer/linux"
	v1beta1 "github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/rig/v2/rigtest"
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

func TestStageBinaries_ValidateStagedBinaries(t *testing.T) {
	const staged = "/tmp/k0s.staged"

	// newHost returns a controller running v1.30 with a binary from a custom k0sBinaryPath staged on it.
	// The staged binary reports stagedVersion.
	newHost := func(t *testing.T, stagedVersion string) *cluster.Host {
		t.Helper()
		mr := rigtest.NewMockRunner()
		mr.ErrDefault = errors.New("unexpected command")
		mr.AddCommand(rigtest.Match("sudo.*true"), func(_ *rigtest.A) error { return nil })
		mr.AddCommandOutput(rigtest.Contains(staged+" version"), stagedVersion+"\n")
		posixFS := remotefs.NewPosixFS(mr)
		client, err := rig.NewClient(
			rig.WithConnection(mr.MockConnection),
			rig.WithRemoteFSProvider(func(_ cmd.Runner) (remotefs.FS, error) {
				return posixFS, nil
			}),
		)
		require.NoError(t, err)
		return &cluster.Host{
			Client:        client,
			Configurer:    &linuxcfg.Ubuntu{},
			Role:          "controller",
			K0sBinaryPath: "/local/path/to/k0s",
			Metadata: cluster.HostMetadata{
				K0sRunningVersion: version.MustParse("v1.30.5+k0s.0"),
				K0sBinaryTempFile: staged,
				NeedsUpgrade:      true,
			},
		}
	}

	t.Run("blocks a staged binary that skips a minor version", func(t *testing.T) {
		h := newHost(t, "v1.32.0+k0s.0")
		installed := h.K0sInstallLocation()
		p := &StageBinaries{hosts: cluster.Hosts{h}}
		err := p.validateStagedBinaries()
		require.ErrorContains(t, err, "from v1.30.5+k0s.0 to v1.32.0+k0s.0")
		require.ErrorContains(t, err, "one minor version at a time")
		require.Equal(t, installed, h.K0sInstallLocation(), "the k0s binary path must be restored")
	})

	t.Run("allows a staged binary one minor version ahead", func(t *testing.T) {
		p := &StageBinaries{hosts: cluster.Hosts{newHost(t, "v1.31.1+k0s.0")}}
		require.NoError(t, p.validateStagedBinaries())
	})

	t.Run("can be skipped", func(t *testing.T) {
		p := &StageBinaries{hosts: cluster.Hosts{newHost(t, "v1.32.0+k0s.0")}, SkipVersionSkewCheck: true}
		require.NoError(t, p.validateStagedBinaries())
	})

	t.Run("does not run the staged binary of a host without a custom k0sBinaryPath", func(t *testing.T) {
		// The version of a downloaded binary is verified against the configured version when it is installed
		h := newHost(t, "not a version")
		h.K0sBinaryPath = ""
		p := &StageBinaries{hosts: cluster.Hosts{h}}
		require.NoError(t, p.validateStagedBinaries())
	})
}
