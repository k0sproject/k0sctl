package phase

import (
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
		files, err := f.Resolve()
		if err != nil {
			return err
		}

		for _, file := range files {
			log.Infof("%s: uploading %s to %s", h, file.Source, file.Destination)

			if err := h.Execf("mkdir -p $(dirname %s)", f.Destination); err != nil {
				return err
			}

			if err := h.Upload(file.Source, file.Destination); err != nil {
				return err
			}

			if err := h.Configurer.Chmod(h, file.Destination, f.PermMode); err != nil {
				return err
			}
		}

	}
	return nil
}
