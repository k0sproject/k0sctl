package airgap

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/adrg/xdg"
	"github.com/k0sproject/k0sctl/configurer"
	linuxcfg "github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

func TestBundleArch(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64", "arm", "riscv64"} {
		got, err := BundleArch(arch)
		require.NoError(t, err)
		require.Equal(t, arch, got)
	}

	_, err := BundleArch("ppc64le")
	require.ErrorContains(t, err, `unsupported airgap bundle architecture "ppc64le"`)
}

func TestGitHubReleaseResolverResolve(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")

	artifact, err := (GitHubReleaseResolver{}).Resolve(k0sVersion, "linux", "amd64")
	require.NoError(t, err)
	require.Equal(t, "k0s-airgap-bundle-v1.34.1+k0s.0-amd64", artifact.Name)
	require.Equal(t, "https://github.com/k0sproject/k0s/releases/download/v1.34.1%2Bk0s.0/k0s-airgap-bundle-v1.34.1+k0s.0-amd64", artifact.URL)
	require.Equal(t, "linux", artifact.OS)
	require.Equal(t, "amd64", artifact.Arch)
}

func TestURLResolverResolveExpandsTokens(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")
	resolver := URLResolver{
		Template: "https://mirror.example.invalid/%o/%p/k0s-%v.tar?token=redacted",
		SHA256:   "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	artifact, err := resolver.Resolve(k0sVersion, "linux", "arm64")
	require.NoError(t, err)
	require.Equal(t, "k0s-v1.34.1+k0s.0.tar", artifact.Name)
	require.Equal(t, "https://mirror.example.invalid/linux/arm64/k0s-v1.34.1%2Bk0s.0.tar?token=redacted", artifact.URL)
	require.Equal(t, "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef", artifact.SHA256)
}

func TestURLResolverRejectsUnsafeArtifactName(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")

	for _, template := range []string{
		"https://mirror.example.invalid/%zz",
		"https://mirror.example.invalid/%o/%p/..",
		"https://mirror.example.invalid/%o/%p/bad:name",
		"https://mirror.example.invalid/%o/%p/bad%3Fname",
		`https://mirror.example.invalid/%o/%p/bad\name`,
	} {
		_, err := (URLResolver{Template: template}).Resolve(k0sVersion, "linux", "amd64")
		require.Error(t, err)
	}
}

func TestURLResolverRedactsURLInArtifactNameErrors(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")

	_, err := (URLResolver{Template: "https://user:pass@mirror.example.invalid/%zz?token=secret"}).Resolve(k0sVersion, "linux", "amd64")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "token=secret")
	require.NotContains(t, err.Error(), "user:pass")

	_, err = (URLResolver{Template: "https://user:pass@mirror.example.invalid/bad:name?token=secret"}).Resolve(k0sVersion, "linux", "amd64")
	require.Error(t, err)
	require.NotContains(t, err.Error(), "token=secret")
	require.NotContains(t, err.Error(), "user:pass")
}

func TestPlanHostsSelectsWorkerCapableLinuxHosts(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")
	hosts := cluster.Hosts{
		host("controller", "amd64", &linuxcfg.Ubuntu{}),
		host("worker", "amd64", &linuxcfg.Ubuntu{}),
		host("controller+worker", "arm64", &linuxcfg.Ubuntu{}),
		host("single", "riscv64", &linuxcfg.Ubuntu{}),
		host("worker", "amd64", &testConfigurer{osKind: "windows"}),
	}
	hosts[2].DataDir = "/opt/k0s"
	hosts[3].Reset = true

	plans, err := PlanHosts(hosts, k0sVersion, GitHubReleaseResolver{})
	require.NoError(t, err)
	require.Len(t, plans, 2)
	require.Equal(t, hosts[1], plans[0].Host)
	require.Equal(t, "/var/lib/k0s/images/k0s-airgap-bundle-v1.34.1+k0s.0-amd64", plans[0].Destination)
	require.Equal(t, hosts[2], plans[1].Host)
	require.Equal(t, "/opt/k0s/images/k0s-airgap-bundle-v1.34.1+k0s.0-arm64", plans[1].Destination)
}

func TestCacheFilePath(t *testing.T) {
	k0sVersion := version.MustParse("v1.34.1+k0s.0")

	got, err := CacheFilePath(k0sVersion, "linux", "amd64", "bundle")
	require.NoError(t, err)
	require.Contains(t, got, filepath.Join("k0sctl", "airgap", "1.34.1+k0s.0", "linux", "amd64", "bundle"))
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "bundle")
	content := []byte("airgap bundle")
	require.NoError(t, os.WriteFile(file, content, 0o644))
	sum := sha256.Sum256(content)

	require.NoError(t, VerifySHA256(file, fmt.Sprintf("%x", sum)))
	require.ErrorContains(t, VerifySHA256(file, "0000"), "sha256 mismatch")
}

func TestEnsureCachedReplacesInvalidCachedBundle(t *testing.T) {
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

	content := []byte("good bundle")
	sum := sha256.Sum256(content)
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, err := w.Write(content)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	artifact := Artifact{
		Name:   "bundle",
		URL:    server.URL + "/bundle",
		OS:     "linux",
		Arch:   "amd64",
		SHA256: fmt.Sprintf("%x", sum),
	}
	cachePath, err := CacheFilePath(k0sVersion, artifact.OS, artifact.Arch, artifact.Name)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cachePath), 0o755))
	require.NoError(t, os.WriteFile(cachePath, []byte("bad bundle"), 0o644))

	got, err := EnsureCached(context.Background(), k0sVersion, artifact)
	require.NoError(t, err)
	require.Equal(t, cachePath, got)
	require.Equal(t, 1, requests)
	require.NoError(t, VerifySHA256(cachePath, artifact.SHA256))
}

func TestEnsureCachedRemovesInvalidDownloadedBundle(t *testing.T) {
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := w.Write([]byte("bad bundle"))
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	artifact := Artifact{
		Name:   "bundle",
		URL:    server.URL + "/bundle",
		OS:     "linux",
		Arch:   "amd64",
		SHA256: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}
	cachePath, err := CacheFilePath(k0sVersion, artifact.OS, artifact.Arch, artifact.Name)
	require.NoError(t, err)

	_, err = EnsureCached(context.Background(), k0sVersion, artifact)
	require.ErrorContains(t, err, "verify downloaded airgap bundle")
	require.NoFileExists(t, cachePath)
}

func TestLocalPath(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")
	require.NoError(t, os.WriteFile(bundle, []byte("data"), 0o644))

	got, err := LocalPath(dir, "bundle")
	require.NoError(t, err)
	require.Equal(t, bundle, got)

	got, err = LocalPath(bundle, "ignored")
	require.NoError(t, err)
	require.Equal(t, bundle, got)
}

type testConfigurer struct {
	linuxcfg.Ubuntu
	osKind string
}

func (c *testConfigurer) OSKind() string {
	return c.osKind
}

func host(role, arch string, cfg configurer.Configurer) *cluster.Host {
	return &cluster.Host{
		Role:       role,
		Configurer: cfg,
		Metadata: cluster.HostMetadata{
			Arch: arch,
		},
	}
}
