package phase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/k0sctl/pkg/download"
	"github.com/k0sproject/rig/v2/remotefs"

	log "github.com/sirupsen/logrus"
)

// UploadFiles implements a phase which upload files to hosts
type UploadFiles struct {
	GenericPhase

	hosts cluster.Hosts
	sums  sumCache
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
	var tracker *download.Tracker
	for _, f := range h.Files {
		if ctx.Err() != nil {
			return fmt.Errorf("upload canceled: %w", ctx.Err())
		}
		var err error
		if f.IsURL() {
			if tracker == nil {
				tracker = hostTracker(h)
			}
			err = p.uploadURL(ctx, h, tracker, f)
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
	if !h.FS().FileExist(dir) {
		targetPerm := perm
		if targetPerm == "" {
			targetPerm = "0755"
		}
		err := p.Wet(h, fmt.Sprintf("create a directory for uploading: `mkdir -p \"%s\"`", dir), func() error {
			if v, perr := strconv.ParseUint(targetPerm, 8, 32); perr == nil {
				return h.Sudo().FS().MkdirAll(dir, fs.FileMode(v))
			}
			return h.Sudo().FS().MkdirAll(dir, fs.FileMode(0o755))
		})
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	if owner != "" {
		err := p.Wet(h, fmt.Sprintf("set owner for directory %s to %s", dir, owner), func() error {
			return h.Sudo().FS().Chown(dir, owner)
		})
		if err != nil {
			return err
		}
	}

	if perm == "" {
		perm = "0755"
	}

	return p.Wet(h, fmt.Sprintf("set permissions for directory %s to %s", dir, perm), func() error {
		return chmodWithString(h, dir, perm)
	})
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

		if f.Sha256 != "" {
			// Checked whether or not this host is about to receive the file, so
			// that a wrong artifact is reported on every apply rather than only
			// on the one that happens to upload it. rig's upload compares the
			// two ends afterwards, which covers the transfer itself.
			if err := p.sums.verify(src, f.Sha256); err != nil {
				return err
			}
		}

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
			err := p.Wet(h, fmt.Sprintf("upload file %s => %s", src, dest), func() error {
				stat, err := os.Stat(src)
				if err != nil {
					return fmt.Errorf("failed to stat local file %s: %w", src, err)
				}
				perm := stat.Mode()
				if s.PermMode != "" {
					if v, perr := strconv.ParseUint(s.PermMode, 8, 32); perr == nil {
						perm = fs.FileMode(v)
					}
				}
				return remotefs.Upload(h.Sudo().FS(), path.Join(f.Base, s.Path), dest, remotefs.WithPermissions(perm))
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
		remoteFile, err := h.Sudo().FS().OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(fileMode))
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

func (p *UploadFiles) uploadURL(ctx context.Context, h *cluster.Host, tracker *download.Tracker, f *cluster.UploadFile) error {
	owner := f.Owner()
	expandedURL := h.ExpandTokens(f.Source, p.Config.Spec.K0s.Version)

	// The destination directory has to exist before the download can be told
	// whether it is already there, and it is created even for a skipped download
	// so that its ownership and permissions still follow the configuration.
	if err := p.ensureDir(h, path.Dir(f.DestinationFile), f.DirPermString, owner); err != nil {
		return err
	}

	verdict := tracker.Needed(ctx, expandedURL, f.DestinationFile, f.Sha256)
	if verdict.Download {
		log.Infof("%s: downloading %s to host %s (%s)", h, f, f.DestinationFile, verdict.Reason)
		// GatherK0sFacts asked the same question earlier and set NeedsUpgrade
		// from the answer. If the answer has changed since, because the content
		// behind the url was replaced between the two, the host is about to
		// receive a new file and the upgrade phases still have to hear about it.
		//
		// Only for a host that is already running k0s: on one that is not, the
		// flag would divert the pending install into the upgrade phases, which
		// have no k0s to upgrade.
		if !h.Reset && !h.Metadata.NeedsUpgrade && h.Metadata.K0sRunningVersion != nil {
			log.Debugf("%s: marking for upgrade because %s is being downloaded to %s", h, expandedURL, f.DestinationFile)
			h.Metadata.NeedsUpgrade = true
		}
		err := p.Wet(h, fmt.Sprintf("download file %s => %s", expandedURL, f.DestinationFile), func() error {
			return tracker.Fetch(ctx, expandedURL, f.DestinationFile, f.Sha256)
		})
		if err != nil {
			return err
		}
	} else {
		log.Infof("%s: %s is already downloaded to %s (%s)", h, f, f.DestinationFile, verdict.Reason)
		if verdict.Adopt && p.IsWet() {
			// The destination was accepted by reading the whole file, which is
			// what every later apply would have to do again without a record of
			// it. Writing one is a change to the host, so it waits for a run
			// that is allowed to make changes.
			tracker.Remember(ctx, expandedURL, f.DestinationFile, f.Sha256)
		}
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
			return h.Sudo().FS().Chown(dest, owner)
		})
		if err != nil {
			return err
		}
	}

	if perm != "" {
		err := p.Wet(h, fmt.Sprintf("set permissions for %s to %s", dest, perm), func() error {
			log.Debugf("%s: setting permissions %s for %s", h, perm, dest)
			return chmodWithString(h, dest, perm)
		})
		if err != nil {
			return err
		}
	}

	if timestamp != nil {
		err := p.Wet(h, fmt.Sprintf("set timestamp for %s to %s", dest, timestamp.String()), func() error {
			log.Debugf("%s: touching %s", h, dest)
			return h.Sudo().FS().Touch(dest, *timestamp)
		})
		if err != nil {
			return fmt.Errorf("failed to touch %s: %w", dest, err)
		}
	}
	return nil
}

func chmodWithString(h *cluster.Host, path, perm string) error {
	mode, err := strconv.ParseUint(perm, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid file mode %q: %w", perm, err)
	}
	return chmodWithMode(h, path, fs.FileMode(mode))
}

func chmodWithMode(h *cluster.Host, path string, mode fs.FileMode) error {
	return h.Sudo().FS().Chmod(path, mode)
}

// sumCache remembers the local files that have been checked against a configured
// sha256 during this run, so that an artifact is read once instead of once per
// host that wants it. Hosts are uploaded to in parallel, hence the lock, which
// is held across the read: two hosts waiting for one checksum beats both of them
// computing it.
type sumCache struct {
	mu     sync.Mutex
	result map[string]error
}

// verify checks a local file against want, at most once per file and sum.
func (c *sumCache) verify(name, want string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := name + "\x00" + want
	if err, ok := c.result[key]; ok {
		return err
	}
	err := verifyLocalSum(name, want)
	if c.result == nil {
		c.result = make(map[string]error)
	}
	c.result[key] = err
	return err
}

// verifyLocalSum reports whether a local file has the sha256 sum the
// configuration says it should have.
func verifyLocalSum(name, want string) error {
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("failed to open %s for checksumming: %w", name, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Debugf("failed to close %s: %s", name, err)
		}
	}()

	digest := sha256.New()
	if _, err := io.Copy(digest, f); err != nil {
		return fmt.Errorf("failed to checksum %s: %w", name, err)
	}
	if sum := hex.EncodeToString(digest.Sum(nil)); !strings.EqualFold(sum, want) {
		return fmt.Errorf("%w: %s: expected sha256 %s, got %s", download.ErrChecksumMismatch, name, want, sum)
	}
	return nil
}
