package phase

import (
	"fmt"
	"path"

	"github.com/alessio/shellescape"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"

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
func (p *UploadFiles) Prepare(config *v1beta1.Cluster) error {
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
		var err error
		if f.IsURL() {
			err = p.uploadURL(h, f)
		} else {
			err = p.uploadFile(h, f)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func ensureDir(h *cluster.Host, dir, perm, owner string) error {
	log.Debugf("%s: ensuring directory %s", h, dir)
	if h.Configurer.FileExist(h, dir) {
		return nil
	}

	if err := h.Configurer.MkDir(h, dir, exec.Sudo(h)); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if perm == "" {
		perm = "0755"
	}

	if err := h.Configurer.Chmod(h, dir, perm, exec.Sudo(h)); err != nil {
		return fmt.Errorf("failed to set permissions for directory %s: %w", dir, err)
	}

	if owner != "" {
		if err := h.Execf(`chown "%s" "%s"`, owner, dir, exec.Sudo(h)); err != nil {
			return err
		}
	}

	return nil
}

func (p *UploadFiles) uploadFile(h *cluster.Host, f *cluster.UploadFile) error {
	log.Infof("%s: uploading %s", h, f)
	numfiles := len(f.Sources)

	for i, s := range f.Sources {
		dest := f.DestinationFile
		if dest == "" {
			dest = path.Join(f.DestinationDir, s.Path)
		}

		src := path.Join(f.Base, s.Path)
		if numfiles > 1 {
			log.Infof("%s: uploading file %s => %s (%d of %d)", h, src, dest, i+1, numfiles)
		}

		owner := f.Owner()

		if err := ensureDir(h, path.Dir(dest), f.DirPermString, owner); err != nil {
			return err
		}

		if err := h.Upload(path.Join(f.Base, s.Path), dest, exec.Sudo(h)); err != nil {
			return err
		}

		if owner != "" {
			log.Debugf("%s: setting owner %s for %s", h, owner, dest)
			if err := h.Execf(`chown %s %s`, shellescape.Quote(owner), shellescape.Quote(dest), exec.Sudo(h)); err != nil {
				return err
			}
		}
		log.Debugf("%s: setting permissions %s for %s", h, s.PermMode, dest)
		if err := h.Configurer.Chmod(h, dest, s.PermMode, exec.Sudo(h)); err != nil {
			return err
		}
	}

	return nil
}

func (p *UploadFiles) uploadURL(h *cluster.Host, f *cluster.UploadFile) error {
	log.Infof("%s: downloading %s to host %s", h, f, f.DestinationFile)
	owner := f.Owner()

	if err := ensureDir(h, path.Dir(f.DestinationFile), f.DirPermString, owner); err != nil {
		return err
	}

	if err := h.Configurer.DownloadURL(h, f.Source, f.DestinationFile, exec.Sudo(h)); err != nil {
		return err
	}

	if f.PermString != "" {
		if err := h.Configurer.Chmod(h, f.DestinationFile, f.PermString, exec.Sudo(h)); err != nil {
			return err
		}
	}

	if owner != "" {
		log.Debugf("%s: setting owner %s for %s", h, owner, f.DestinationFile)
		if err := h.Execf(`chown %s %s`, shellescape.Quote(owner), shellescape.Quote(f.DestinationFile), exec.Sudo(h)); err != nil {
			return err
		}
	}

	return nil
}
