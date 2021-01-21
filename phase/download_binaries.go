package phase

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

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
	return "Download binaries"
}

// Prepare the phase
func (p *DownloadBinaries) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return h.UploadBinary
	})
	return nil
}

// ShouldRun is true when the phase should be run
func (p *DownloadBinaries) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *DownloadBinaries) Run() error {
	binaries := make(map[string]string)
	for _, h := range p.hosts {
		if binaries[h.Metadata.Arch] == "" {
			var found string
			for _, b := range p.Config.Spec.Hosts {
				if b.Metadata.Arch == h.Metadata.Arch && b.K0sBinaryPath != "" {
					log.Infof("%s: using binary %s for %s", h, b.K0sBinaryPath, b.Metadata.Arch)
					found = b.K0sBinaryPath
					break
				}
			}
			binaries[h.Metadata.Arch] = found
		}
	}

	for k, v := range binaries {
		if v == "" {
			path, err := p.download(k)
			if err != nil {
				return err
			}
			binaries[k] = path
		}
	}

	for _, h := range p.hosts {
		if h.K0sBinaryPath == "" {
			h.K0sBinaryPath = binaries[h.Metadata.Arch]
		}
	}

	return nil
}

func (p *DownloadBinaries) download(arch string) (string, error) {
	log.Infof("downloading k0s binary for %s", arch)

	version := p.Config.Spec.K0s.Version
	url := fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/v%s/k0s-v%s-%s", version, version, arch)

	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if err == nil {
			return "", fmt.Errorf("Failed to get k0s binary (%d)", resp.StatusCode)
		}
		return "", err
	}

	out, err := ioutil.TempFile("", "k0s")
	if err != nil {
		return "", err
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	fmt.Println(out.Close())

	return out.Name(), nil
}
