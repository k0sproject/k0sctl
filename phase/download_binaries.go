package phase

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// DownloadBinaries downloads k0s binaries to localohost temp files
type DownloadBinaries struct {
	GenericPhase
	hosts []*cluster.Host
}

// Title for the phase
func (p *DownloadBinaries) Title() string {
	return "Download k0s binaries to local host"
}

// Prepare the phase
func (p *DownloadBinaries) Prepare(config *v1beta1.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return !h.Reset && h.UploadBinary && !h.Metadata.K0sBinaryVersion.Equal(config.Spec.K0s.Version)
	})
	return nil
}

// ShouldRun is true when the phase should be run
func (p *DownloadBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *DownloadBinaries) Run(_ context.Context) error {
	var bins binaries

	for _, h := range p.hosts {
		if bin := bins.find(h.Configurer.Kind(), h.Metadata.Arch); bin != nil {
			continue
		}

		bin := &binary{arch: h.Metadata.Arch, os: h.Configurer.Kind(), version: p.Config.Spec.K0s.Version}

		// find configuration defined binpaths and use instead of downloading a new one
		for _, v := range p.hosts {
			if v.Metadata.Arch == bin.arch && v.Configurer.Kind() == bin.os && v.K0sBinaryPath != "" {
				bin.path = h.K0sBinaryPath
			}
		}

		bins = append(bins, bin)
	}

	for _, bin := range bins {
		if bin.path != "" {
			continue
		}
		if err := bin.download(); err != nil {
			return err
		}
	}

	for _, h := range p.hosts {
		if h.K0sBinaryPath == "" {
			if bin := bins.find(h.Configurer.Kind(), h.Metadata.Arch); bin != nil {
				h.UploadBinaryPath = bin.path
			}
		} else {
			h.UploadBinaryPath = h.K0sBinaryPath
		}
	}

	return nil
}

type binary struct {
	arch    string
	os      string
	version *version.Version
	path    string
}

func (b *binary) download() error {
	fn := path.Join("k0sctl", "k0s", b.os, b.arch, "k0s-"+strings.TrimPrefix(b.version.String(), "v")+b.ext())
	p, err := xdg.SearchCacheFile(fn)
	if err == nil {
		b.path = p
		return nil
	}
	p, err = xdg.CacheFile(fn)
	if err != nil {
		return err
	}
	if err := b.downloadTo(p); err != nil {
		return err
	}

	b.path = p
	log.Infof("using k0s binary from %s for %s-%s", b.path, b.os, b.arch)

	return nil
}

func (b binary) ext() string {
	if b.os == "windows" {
		return ".exe"
	}
	return ""
}

func (b binary) url() string {
	v := strings.ReplaceAll(strings.TrimPrefix(b.version.String(), "v"), "+", "%2B")
	return fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/v%[1]s/k0s-v%[1]s-%[2]s%[3]s", v, b.arch, b.ext())
}

func (b binary) downloadTo(path string) error {
	log.Infof("downloading k0s version %s binary for %s-%s from %s", b.version, b.os, b.arch, b.url())

	var err error

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			err = os.Remove(path)
			if err != nil {
				log.Warnf("failed to remove broken download at %s: %s", path, err.Error())
			}
		}
	}()

	resp, err := http.Get(b.url())
	if err != nil {
		return err
	}
	respBody := resp.Body
	defer func() {
		if err := respBody.Close(); err != nil {
			log.Warnf("failed to close download response body for %s: %v", b.url(), err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get k0s binary (http %d)", resp.StatusCode)
	}

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	log.Infof("cached k0s binary to %s", path)

	return nil
}

type binaries []*binary

func (b binaries) find(os, arch string) *binary {
	for _, v := range b {
		if v.arch == arch && v.os == os {
			return v
		}
	}
	return nil
}
