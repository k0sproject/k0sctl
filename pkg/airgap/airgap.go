package airgap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
	"github.com/k0sproject/k0sctl/internal/download"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// Artifact describes an airgap bundle artifact for a host platform.
type Artifact struct {
	Name   string
	URL    string
	OS     string
	Arch   string
	SHA256 string
}

// Plan describes one host's airgap bundle placement.
type Plan struct {
	Host        *cluster.Host
	Artifact    Artifact
	LocalPath   string
	Destination string
}

// Resolver resolves airgap bundle artifacts for a platform.
type Resolver interface {
	Resolve(k0sVersion *version.Version, osKind, arch string) (Artifact, error)
}

// GitHubReleaseResolver resolves official k0s release airgap bundles.
type GitHubReleaseResolver struct{}

// BundleName returns the official k0s airgap bundle filename.
func BundleName(k0sVersion *version.Version, arch string) (string, error) {
	if k0sVersion == nil || k0sVersion.IsZero() {
		return "", errors.New("k0s version is required")
	}
	platform, err := BundleArch(arch)
	if err != nil {
		return "", err
	}
	return bundleNameForPlatform(k0sVersion, platform), nil
}

func bundleNameForPlatform(k0sVersion *version.Version, platform string) string {
	return fmt.Sprintf("k0s-airgap-bundle-%s-%s", k0sVersion.String(), platform)
}

// BundleArch maps host architectures to released k0s airgap bundle architectures.
func BundleArch(arch string) (string, error) {
	switch arch {
	case "amd64", "arm64", "arm", "riscv64":
		return arch, nil
	default:
		return "", fmt.Errorf("unsupported airgap bundle architecture %q", arch)
	}
}

// Resolve resolves an official k0s release artifact.
func (GitHubReleaseResolver) Resolve(k0sVersion *version.Version, osKind, arch string) (Artifact, error) {
	if osKind != "linux" {
		return Artifact{}, fmt.Errorf("unsupported airgap bundle OS %q", osKind)
	}
	platform, err := BundleArch(arch)
	if err != nil {
		return Artifact{}, err
	}
	if k0sVersion == nil || k0sVersion.IsZero() {
		return Artifact{}, errors.New("k0s version is required")
	}
	name := bundleNameForPlatform(k0sVersion, platform)
	return Artifact{
		Name: name,
		URL:  fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/%s/%s", url.QueryEscape(k0sVersion.String()), name),
		OS:   osKind,
		Arch: platform,
	}, nil
}

// URLResolver resolves custom URL-template artifacts.
type URLResolver struct {
	Template string
	SHA256   string
}

// Resolve resolves a custom URL-template artifact.
func (r URLResolver) Resolve(k0sVersion *version.Version, osKind, arch string) (Artifact, error) {
	if osKind != "linux" {
		return Artifact{}, fmt.Errorf("unsupported airgap bundle OS %q", osKind)
	}
	platform, err := BundleArch(arch)
	if err != nil {
		return Artifact{}, err
	}
	if k0sVersion == nil || k0sVersion.IsZero() {
		return Artifact{}, errors.New("k0s version is required")
	}
	name := bundleNameForPlatform(k0sVersion, platform)
	expanded := ExpandURLTemplate(r.Template, k0sVersion, osKind, platform)
	artifactName, err := artifactNameFromURL(expanded)
	if err != nil {
		return Artifact{}, err
	}
	if artifactName == "" {
		artifactName = name
	}
	return Artifact{
		Name:   artifactName,
		URL:    expanded,
		OS:     osKind,
		Arch:   platform,
		SHA256: r.SHA256,
	}, nil
}

func artifactNameFromURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse artifact URL %q: %w", download.RedactedURL(rawURL), urlParseCause(err))
	}
	if parsed.Path == "" {
		return "", nil
	}
	artifactName := path.Base(parsed.Path)
	if artifactName == "." || artifactName == "/" {
		return "", nil
	}
	if err := validateArtifactName(artifactName); err != nil {
		return "", fmt.Errorf("artifact name from URL %q: %w", download.RedactedURL(rawURL), err)
	}
	return artifactName, nil
}

func urlParseCause(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}

func validateArtifactName(name string) error {
	if name == "" {
		return errors.New("artifact name is required")
	}
	if name == ".." || strings.ContainsAny(name, `<>:"/\|?*`) {
		return fmt.Errorf("invalid artifact name %q", name)
	}
	for _, r := range name {
		if r < 0x20 {
			return fmt.Errorf("invalid artifact name %q", name)
		}
	}
	return nil
}

// ExpandURLTemplate expands k0s-style URL tokens.
func ExpandURLTemplate(template string, k0sVersion *version.Version, osKind, arch string) string {
	var versionString string
	if k0sVersion != nil {
		versionString = url.QueryEscape(k0sVersion.String())
	}
	replacer := strings.NewReplacer(
		"%%", "\x00",
		"%v", versionString,
		"%p", arch,
		"%o", osKind,
		"\x00", "%",
	)
	return replacer.Replace(template)
}

// Destination returns the default bundle destination for a host.
func Destination(h *cluster.Host, artifactName string) string {
	return path.Join(h.K0sDataDir(), "images", artifactName)
}

func isWorkerCapable(h *cluster.Host) bool {
	switch h.Role {
	case "worker", "controller+worker", "single":
		return true
	default:
		return false
	}
}

// PlanHosts creates airgap placement plans for hosts.
func PlanHosts(hosts cluster.Hosts, k0sVersion *version.Version, resolver Resolver) ([]Plan, error) {
	var plans []Plan
	for _, h := range hosts {
		if h.Reset || !isWorkerCapable(h) {
			continue
		}
		osKind, err := h.OSKind()
		if err != nil {
			return nil, fmt.Errorf("%s: get OS kind: %w", h, err)
		}
		if osKind != "linux" {
			continue
		}
		arch, err := h.Arch()
		if err != nil {
			return nil, fmt.Errorf("%s: get architecture: %w", h, err)
		}
		artifact, err := resolver.Resolve(k0sVersion, osKind, arch)
		if err != nil {
			return nil, fmt.Errorf("%s: resolve airgap bundle: %w", h, err)
		}
		if err := validateArtifactName(artifact.Name); err != nil {
			return nil, fmt.Errorf("%s: resolve airgap bundle: %w", h, err)
		}
		plans = append(plans, Plan{
			Host:        h,
			Artifact:    artifact,
			Destination: Destination(h, artifact.Name),
		})
	}
	return plans, nil
}

// CacheFilePath returns the XDG cache path for an airgap artifact.
func CacheFilePath(k0sVersion *version.Version, osKind, arch, artifactName string) (string, error) {
	if k0sVersion == nil || k0sVersion.IsZero() {
		return "", errors.New("k0s version is required")
	}
	if err := validateArtifactName(artifactName); err != nil {
		return "", err
	}
	fn := path.Join("k0sctl", "airgap", strings.TrimPrefix(k0sVersion.String(), "v"), osKind, arch, artifactName)
	if cached, err := xdg.SearchCacheFile(fn); err == nil {
		return cached, nil
	}
	return xdg.CacheFile(fn)
}

// EnsureCached downloads an artifact to the local XDG cache when needed.
func EnsureCached(ctx context.Context, k0sVersion *version.Version, artifact Artifact) (string, error) {
	dest, err := CacheFilePath(k0sVersion, artifact.OS, artifact.Arch, artifact.Name)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(dest); err == nil {
		if artifact.SHA256 != "" {
			if err := VerifySHA256(dest, artifact.SHA256); err != nil {
				log.Warnf("cached airgap bundle %s failed checksum verification, removing it: %v", dest, err)
				if removeErr := os.Remove(dest); removeErr != nil {
					return "", fmt.Errorf("remove invalid cached airgap bundle %s after checksum failure: %w", dest, removeErr)
				}
			} else {
				return dest, nil
			}
		} else {
			return dest, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat airgap cache path %s: %w", dest, err)
	}
	if artifact.URL == "" {
		return "", errors.New("artifact URL is required")
	}
	log.Infof("downloading k0s airgap bundle %s for %s-%s", artifact.Name, artifact.OS, artifact.Arch)
	if err := download.ToFile(ctx, artifact.URL, dest); err != nil {
		return "", fmt.Errorf("download airgap bundle: %w", err)
	}
	if artifact.SHA256 != "" {
		if err := VerifySHA256(dest, artifact.SHA256); err != nil {
			if removeErr := os.Remove(dest); removeErr != nil && !os.IsNotExist(removeErr) {
				return "", fmt.Errorf("remove invalid downloaded airgap bundle %s after checksum failure: %w", dest, removeErr)
			}
			return "", fmt.Errorf("verify downloaded airgap bundle: %w", err)
		}
	}
	return dest, nil
}

// VerifySHA256 checks a file against an expected SHA-256 hex digest.
func VerifySHA256(filePath, expected string) error {
	expected = strings.TrimSpace(strings.ToLower(expected))
	if expected == "" {
		return nil
	}
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Warnf("failed to close %s: %v", filePath, err)
		}
	}()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return fmt.Errorf("sha256 mismatch for %s: got %s, want %s", filePath, actual, expected)
	}
	return nil
}

// LocalPath resolves a local airgap source path for an artifact.
func LocalPath(sourcePath, artifactName string) (string, error) {
	stat, err := os.Stat(sourcePath)
	if err != nil {
		return "", err
	}
	if stat.IsDir() {
		return filepath.Join(sourcePath, artifactName), nil
	}
	return sourcePath, nil
}
