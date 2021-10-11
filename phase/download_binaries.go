package phase

import (
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/k0sproject/k0sctl/cache"
	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"
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
func (p *DownloadBinaries) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.UploadBinary && h.Metadata.K0sBinaryVersion != config.Spec.K0s.Version
	})
	return nil
}

// ShouldRun is true when the phase should be run
func (p *DownloadBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *DownloadBinaries) Run() error {
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
	version string
	path    string
}

func (b *binary) download() error {
	path, err := cache.GetOrCreate(b.downloadTo, "k0s", b.os, b.arch, "k0s-"+b.version+b.ext())
	if err != nil {
		return err
	}

	b.path = path
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
	return fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/v%s/k0s-v%s-%s%s", b.version, b.version, b.arch, b.ext())
}

func (b binary) downloadTo(path string) error {
	log.Infof("downloading k0s version %s binary for %s-%s", b.version, b.os, b.arch)

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
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get k0s binary (http %d)", resp.StatusCode)
	}

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	if err = f.Close(); err == nil {
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
