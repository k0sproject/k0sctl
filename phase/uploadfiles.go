package phase

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"time"

	"al.essio.dev/pkg/shellescape"
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
func (p *UploadFiles) Run(ctx context.Context) error {
	return p.parallelDoUpload(ctx, p.Config.Spec.Hosts, p.uploadFiles)
}

func (p *UploadFiles) uploadFiles(ctx context.Context, h *cluster.Host) error {
	for _, f := range h.Files {
		if ctx.Err() != nil {
			return fmt.Errorf("upload canceled: %w", ctx.Err())
		}
		var err error
		if f.IsURL() {
			err = p.uploadURL(h, f)
		} else if len(f.Sources) > 0 {
			err = p.uploadFile(h, f)
		} else if f.HasData() {
			err = p.uploadData(h, f)
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

		var stat os.FileInfo
		var err error
		if h.FileChanged(src, dest) {
			stat, err = os.Stat(src)
			if err != nil {
				return fmt.Errorf("failed to stat local file %s: %w", src, err)
			}
			err = p.Wet(h, fmt.Sprintf("upload file %s => %s", src, dest), func() error {
				return h.Upload(path.Join(f.Base, s.Path), dest, stat.Mode(), exec.Sudo(h), exec.LogError(true))
			})
			if err != nil {
				return err
			}
		} else {
			log.Infof("%s: file already exists and hasn't been changed, skipping upload", h)
		}

		if stat == nil {
			stat, err = os.Stat(src)
			if err != nil {
				return fmt.Errorf("failed to stat %s: %w", src, err)
			}
		}
		modTime := stat.ModTime()
		if err := p.applyFileMetadata(h, dest, owner, s.PermMode, &modTime); err != nil {
			return err
		}
	}

	return nil
}

func (p *UploadFiles) uploadData(h *cluster.Host, f *cluster.UploadFile) error {
	log.Infof("%s: uploading inline data", h)
	dest := f.DestinationFile
	if dest == "" {
		if f.DestinationDir != "" {
			dest = path.Join(f.DestinationDir, f.Name)
		} else {
			dest = f.Name
		}
	}

	owner := f.Owner()

	if err := p.ensureDir(h, path.Dir(dest), f.DirPermString, owner); err != nil {
		return err
	}

	err := p.Wet(h, fmt.Sprintf("upload inline data => %s", dest), func() error {
		fileMode, _ := strconv.ParseUint(f.PermString, 8, 32)
		remoteFile, err := h.SudoFsys().OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(fileMode))
		if err != nil {
			return err
		}

		defer func() {
			if err := remoteFile.Close(); err != nil {
				log.Warnf("failed to close remote file %s: %v", dest, err)
			}
		}()

		_, err = fmt.Fprint(remoteFile, f.Data)

		return err
	})
	if err != nil {
		return err
	}

	return p.applyFileMetadata(h, dest, owner, "", nil)
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

	perm := ""
	if f.PermString != "" {
		perm = f.PermString
	}

	return p.applyFileMetadata(h, f.DestinationFile, owner, perm, nil)
}

func (p *UploadFiles) applyFileMetadata(h *cluster.Host, dest, owner, perm string, timestamp *time.Time) error {
	if owner != "" {
		err := p.Wet(h, fmt.Sprintf("set owner for %s to %s", dest, owner), func() error {
			log.Debugf("%s: setting owner %s for %s", h, owner, dest)
			return h.Execf(`chown %s %s`, shellescape.Quote(owner), shellescape.Quote(dest), exec.Sudo(h))
		})
		if err != nil {
			return err
		}
	}

	if perm != "" {
		err := p.Wet(h, fmt.Sprintf("set permissions for %s to %s", dest, perm), func() error {
			log.Debugf("%s: setting permissions %s for %s", h, perm, dest)
			return h.Configurer.Chmod(h, dest, perm, exec.Sudo(h))
		})
		if err != nil {
			return err
		}
	}

	if timestamp != nil {
		err := p.Wet(h, fmt.Sprintf("set timestamp for %s to %s", dest, timestamp.String()), func() error {
			log.Debugf("%s: touching %s", h, dest)
			return h.Configurer.Touch(h, dest, *timestamp, exec.Sudo(h))
		})
		if err != nil {
			return fmt.Errorf("failed to touch %s: %w", dest, err)
		}
	}

	return nil
}
