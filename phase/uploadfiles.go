package phase

import (
	"path/filepath"

	"github.com/k0sproject/k0sctl/config"
	"github.com/k0sproject/k0sctl/config/cluster"

	log "github.com/sirupsen/logrus"
)

// UploadFiles implements a phase which upload files to hosts
type UploadFiles struct {
	GenericPhase

	hosts cluster.Hosts
}

// Title for the phase
func (p *UploadFiles) Title() string {
	return "Upload files to hosts"
}

// Prepare the phase
func (p *UploadFiles) Prepare(config *config.Cluster) error {
	p.Config = config
	p.hosts = p.Config.Spec.Hosts.Filter(func(h *cluster.Host) bool {
		return len(h.Files) > 0
	})

	return nil
}

// ShouldRun is true when there are workers
func (p *UploadFiles) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UploadFiles) Run() error {
	return p.Config.Spec.Hosts.ParallelEach(p.uploadFiles)
}

func (p *UploadFiles) uploadFiles(h *cluster.Host) error {
	for _, f := range h.Files {
		log.Infof("%s: starting to upload %s", h, f.Name)
		files, err := f.Resolve()
		if err != nil {
			return err
		}

		if err := h.Execf("install -d %s -m %s", f.DestinationDir, f.PermMode); err != nil {
			return err
		}

		for _, file := range files {
			log.Debugf("%s: uploading %s to %s", h, file, f.DestinationDir)
			destination := filepath.Join(f.DestinationDir, filepath.Base(file))

			if err := h.Upload(file, destination); err != nil {
				return err
			}

			if err := h.Configurer.Chmod(h, destination, f.PermMode); err != nil {
				return err
			}
		}
		log.Infof("%s: %s upload done", h, f.Name)
	}
	return nil
}
