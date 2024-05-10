package phase

import (
	"fmt"
	"os"
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
		return !h.Reset && len(h.Files) > 0
	})

	return nil
}

// ShouldRun is true when there are workers
func (p *UploadFiles) ShouldRun() bool {
	return len(p.hosts) > 0
}

// Run the phase
func (p *UploadFiles) Run() error {
	return p.parallelDoUpload(p.Config.Spec.Hosts, p.uploadFiles)
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

func (p *UploadFiles) ensureDir(h *cluster.Host, dir, perm, owner string) error {
	log.Debugf("%s: ensuring directory %s", h, dir)
	if h.Configurer.FileExist(h, dir) {
		return nil
	}

	err := p.Wet(h, fmt.Sprintf("create a directory for uploading: `mkdir -p \"%s\"`", dir), func() error {
		return h.Configurer.MkDir(h, dir, exec.Sudo(h))
	})
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	if perm == "" {
		perm = "0755"
	}

	err = p.Wet(h, fmt.Sprintf("set permissions for directory %s to %s", dir, perm), func() error {
		return h.Configurer.Chmod(h, dir, perm, exec.Sudo(h))
	})
	if err != nil {
		return fmt.Errorf("failed to set permissions for directory %s: %w", dir, err)
	}

	if owner != "" {
		err = p.Wet(h, fmt.Sprintf("set owner for directory %s to %s", dir, owner), func() error {
			return h.Execf(`chown "%s" "%s"`, owner, dir, exec.Sudo(h))
		})
		if err != nil {
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

		if err := p.ensureDir(h, path.Dir(dest), f.DirPermString, owner); err != nil {
			return err
		}

		if h.FileChanged(src, dest) {
			err := p.Wet(h, fmt.Sprintf("upload file %s => %s", src, dest), func() error {
				return h.Upload(path.Join(f.Base, s.Path), dest, exec.Sudo(h), exec.LogError(true))
			})
			if err != nil {
				return err
			}
		} else {
			log.Infof("%s: file already exists and hasn't been changed, skipping upload", h)
		}

		if owner != "" {
			err := p.Wet(h, fmt.Sprintf("set owner for %s to %s", dest, owner), func() error {
				log.Debugf("%s: setting owner %s for %s", h, owner, dest)
				return h.Execf(`chown %s %s`, shellescape.Quote(owner), shellescape.Quote(dest), exec.Sudo(h))
			})
			if err != nil {
				return err
			}
		}
		err := p.Wet(h, fmt.Sprintf("set permissions for %s to %s", dest, s.PermMode), func() error {
			log.Debugf("%s: setting permissions %s for %s", h, s.PermMode, dest)
			return h.Configurer.Chmod(h, dest, s.PermMode, exec.Sudo(h))
		})
		if err != nil {
			return err
		}
		stat, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %s", src, err)
		}
		err = p.Wet(h, fmt.Sprintf("set timestamp for %s to %s", dest, stat.ModTime()), func() error {
			log.Debugf("%s: touching %s", h, dest)
			return h.Configurer.Touch(h, dest, stat.ModTime(), exec.Sudo(h))
		})
		if err != nil {
			return fmt.Errorf("failed to touch %s: %w", dest, err)
		}
	}

	return nil
}

func (p *UploadFiles) uploadURL(h *cluster.Host, f *cluster.UploadFile) error {
	log.Infof("%s: downloading %s to host %s", h, f, f.DestinationFile)
	owner := f.Owner()

	if err := p.ensureDir(h, path.Dir(f.DestinationFile), f.DirPermString, owner); err != nil {
		return err
	}

	expandedURL := h.ExpandTokens(f.Source, p.Config.Spec.K0s.Version)
	err := p.Wet(h, fmt.Sprintf("download file %s => %s", expandedURL, f.DestinationFile), func() error {
		return h.Configurer.DownloadURL(h, expandedURL, f.DestinationFile, exec.Sudo(h))
	})
	if err != nil {
		return err
	}

	if f.PermString != "" {
		err := p.Wet(h, fmt.Sprintf("set permissions for %s to %s", f.DestinationFile, f.PermString), func() error {
			return h.Configurer.Chmod(h, f.DestinationFile, f.PermString, exec.Sudo(h))
		})
		if err != nil {
			return err
		}
	}

	if owner != "" {
		err := p.Wet(h, fmt.Sprintf("set owner for %s to %s", f.DestinationFile, owner), func() error {
			log.Debugf("%s: setting owner %s for %s", h, owner, f.DestinationFile)
			return h.Execf(`chown %s %s`, shellescape.Quote(owner), shellescape.Quote(f.DestinationFile), exec.Sudo(h))
		})
		if err != nil {
			return err
		}
	}

	return nil
}
