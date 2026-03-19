package binprovider

import (
	"context"
	"errors"

	"github.com/k0sproject/k0sctl/pkg/k0s"
	"github.com/k0sproject/version"
)

// downloadProvider downloads a k0s binary directly on the remote host.
// The URL is resolved at Stage time via urlFor.
type downloadProvider struct {
	stagedFile
	installPath string
	urlFor      func(*version.Version) (string, error)
	target      *version.Version
}

func (p *downloadProvider) IsUpload() bool { return false }

func (p *downloadProvider) NeedsUpgrade() bool {
	return needsUpgrade(p.host, p.target, p.host.InstalledK0sVersion(), p.host.RunningK0sVersion())
}

func (p *downloadProvider) Stage(_ context.Context) (string, error) {
	if p.target == nil {
		return "", errors.New("no target version set")
	}
	url, err := p.urlFor(p.target)
	if err != nil {
		return "", err
	}
	tmp, err := stageDownload(p.host, url, p.installPath, p.target)
	if err != nil {
		return "", err
	}
	p.tmpPath = tmp
	return tmp, nil
}

// NewGitHub returns a BinaryProvider that fetches the k0s binary from GitHub.
func NewGitHub(h Host, installPath string, target *version.Version) k0s.BinaryProvider {
	return &downloadProvider{
		stagedFile:  stagedFile{host: h},
		installPath: installPath,
		target:      target,
		urlFor: func(v *version.Version) (string, error) {
			arch, err := h.Arch()
			if err != nil {
				return "", err
			}
			osKind, err := h.OSKind()
			if err != nil {
				return "", err
			}
			return v.DownloadURL(osKind, arch), nil
		},
	}
}

// NewCustomURL returns a BinaryProvider that downloads k0s from a user-supplied URL.
// urlFor is called at Stage time to resolve the final URL for the target version.
func NewCustomURL(h Host, installPath string, urlFor func(*version.Version) string, target *version.Version) k0s.BinaryProvider {
	return &downloadProvider{
		stagedFile:  stagedFile{host: h},
		installPath: installPath,
		target:      target,
		urlFor:      func(v *version.Version) (string, error) { return urlFor(v), nil },
	}
}
