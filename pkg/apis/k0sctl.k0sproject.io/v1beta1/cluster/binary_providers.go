package cluster

import (
	k0s "github.com/k0sproject/k0sctl/pkg/k0s"
	"github.com/k0sproject/k0sctl/pkg/k0s/binprovider"
	"github.com/k0sproject/version"
)

// defaultBinaryProvider returns the appropriate built-in BinaryProvider for the host
// based on its configuration fields (UseExistingK0s, K0sBinaryPath, UploadBinary, K0sDownloadURLOverride, etc.).
func (h *Host) defaultBinaryProvider(target *version.Version) k0s.BinaryProvider {
	if h.UseExistingK0s {
		return binprovider.NewExisting(h)
	}
	if h.K0sBinaryPath != "" {
		installPath := h.K0sInstallLocation()
		return binprovider.NewLocalFile(h, h.K0sBinaryPath, installPath, func() bool {
			// If the host or its configurer is not yet initialized, treat the binary as "changed"
			// to avoid dereferencing a nil Configurer in FileChanged.
			if h == nil || h.Configurer == nil {
				return true
			}
			return h.FileChanged(h.K0sBinaryPath, installPath)
		})
	}
	if h.UploadBinary {
		return binprovider.NewLocalUpload(h, h.K0sInstallLocation(), target)
	}
	if h.K0sDownloadURLOverride != "" {
		return binprovider.NewCustomURL(h, h.K0sInstallLocation(), func(v *version.Version) string {
			return h.ExpandTokens(h.K0sDownloadURLOverride, v)
		}, target)
	}
	return binprovider.NewGitHub(h, h.K0sInstallLocation(), target)
}
