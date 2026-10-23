package binprovider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"github.com/k0sproject/k0sctl/internal/download"
	"github.com/k0sproject/k0sctl/pkg/k0s"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// localUpload downloads a k0s binary to a local cache and uploads it to the host.
type localUpload struct {
	stagedFile
	installPath string
	target      *version.Version
}

var _ k0s.BinaryCacher = (*localUpload)(nil)

func cacheFilePath(osKind, arch string, v *version.Version) (string, error) {
	ext := ""
	if osKind == "windows" {
		ext = ".exe"
	}
	fn := path.Join("k0sctl", "k0s", osKind, arch, "k0s-"+strings.TrimPrefix(v.String(), "v")+ext)
	if cached, err := xdg.SearchCacheFile(fn); err == nil {
		return cached, nil
	}
	return xdg.CacheFile(fn)
}

func (p *localUpload) BinaryCacheKey() (string, error) {
	if p.target == nil {
		return "", errors.New("no target version set")
	}
	arch, err := p.host.Arch()
	if err != nil {
		return "", fmt.Errorf("get host arch: %w", err)
	}
	osKind, err := p.host.OSKind()
	if err != nil {
		return "", fmt.Errorf("get host os kind: %w", err)
	}
	return cacheFilePath(osKind, arch, p.target)
}

func (p *localUpload) EnsureCached(ctx context.Context) error {
	if p.target == nil {
		return errors.New("no target version set")
	}
	arch, err := p.host.Arch()
	if err != nil {
		return fmt.Errorf("get host arch: %w", err)
	}
	osKind, err := p.host.OSKind()
	if err != nil {
		return fmt.Errorf("get host os kind: %w", err)
	}
	dest, err := cacheFilePath(osKind, arch, p.target)
	if err != nil {
		return fmt.Errorf("prepare k0s cache path: %w", err)
	}
	if _, err := os.Stat(dest); err == nil {
		log.Debugf("using cached k0s %s binary for %s-%s from %s", p.target, osKind, arch, dest)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat k0s cache path %s: %w", dest, err)
	}
	url := p.target.DownloadURL(osKind, arch)
	log.Infof("downloading k0s %s binary for %s-%s", p.target, osKind, arch)
	if err := download.ToFile(ctx, url, dest); err != nil {
		return fmt.Errorf("download k0s binary: %w", err)
	}
	log.Debugf("cached k0s binary to %s", dest)
	return nil
}

func (p *localUpload) IsUpload() bool { return true }

func (p *localUpload) NeedsUpgrade() bool {
	return needsUpgrade(p.host, p.target, p.host.InstalledK0sVersion(), p.host.RunningK0sVersion())
}

func (p *localUpload) Stage(ctx context.Context) (string, error) {
	if p.target == nil {
		return "", errors.New("no target version set")
	}
	arch, err := p.host.Arch()
	if err != nil {
		return "", fmt.Errorf("get host arch: %w", err)
	}
	osKind, err := p.host.OSKind()
	if err != nil {
		return "", fmt.Errorf("get host os kind: %w", err)
	}
	cachePath, err := cacheFilePath(osKind, arch, p.target)
	if err != nil {
		return "", fmt.Errorf("locate k0s cache: %w", err)
	}
	if _, err := os.Stat(cachePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("k0s binary not in local cache at %s: EnsureCached must be called first", cachePath)
		}
		return "", fmt.Errorf("stat k0s cache path: %w", err)
	}
	tmp, err := stageUpload(p.host, cachePath, p.installPath)
	if err != nil {
		return "", err
	}
	p.tmpPath = tmp
	return tmp, nil
}

// NewLocalUpload returns a BinaryProvider that downloads k0s to a local cache and uploads it to the host.
func NewLocalUpload(h Host, installPath string, target *version.Version) k0s.BinaryProvider {
	return &localUpload{stagedFile: stagedFile{host: h}, installPath: installPath, target: target}
}
