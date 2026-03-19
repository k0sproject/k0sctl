package binprovider

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/version"
	log "github.com/sirupsen/logrus"
)

// stagedFile is embedded by providers that place a temporary binary on the remote host.
// It holds the host reference and the tmpPath set during Stage, and provides CleanUp.
type stagedFile struct {
	host    Host
	tmpPath string
}

// CleanUp removes the temporary binary placed by Stage, if any.
func (s *stagedFile) CleanUp(_ context.Context) {
	if s.tmpPath == "" {
		return
	}
	if err := s.host.DeleteFile(s.tmpPath); err != nil {
		log.Debugf("%s: failed to clean up k0s binary temp file: %v", s.host, err)
	}
	s.tmpPath = ""
}

// needsUpgrade is the shared NeedsUpgrade logic for providers that install a specific
// version: prefer the running version for comparison; fall back to the on-disk binary;
// if neither is known, an upgrade is always needed.
func needsUpgrade(h Host, target, binary, running *version.Version) bool {
	if target == nil {
		log.Debugf("%s: needs binary (target version unknown)", h)
		return true
	}
	if running == nil {
		if binary == nil {
			log.Debugf("%s: needs binary (no installed or running k0s found, target=%s)", h, target)
			return true
		}
		result := !target.Equal(binary)
		log.Debugf("%s: installed=%s target=%s running=none → needsUpgrade=%v", h, binary, target, result)
		return result
	}
	result := !target.Equal(running)
	log.Debugf("%s: running=%s target=%s → needsUpgrade=%v", h, running, target, result)
	return result
}

// stageTempPath returns a temp file path for the k0s binary on the host,
// preserving the .exe extension on Windows so the file remains executable.
func stageTempPath(h Host, bin string) string {
	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	if h.IsWindows() {
		if ext := filepath.Ext(bin); strings.EqualFold(ext, ".exe") {
			return strings.TrimSuffix(bin, ext) + ".tmp." + ts + ext
		}
	}
	return bin + ".tmp." + ts
}

// stageUpload uploads a local file to a temporary path next to the install location on
// the remote host. It creates the parent directory, uploads with 0o600 permissions,
// preserves the source modification time, and sets the executable bit (non-Windows).
func stageUpload(h Host, src, installPath string) (tmp string, err error) {
	tmp = stageTempPath(h, installPath)

	uploaded := false
	defer func() {
		if err != nil && uploaded {
			if rmErr := h.DeleteFile(tmp); rmErr != nil {
				log.Debugf("%s: failed to remove staged temp binary %s: %v", h, tmp, rmErr)
			}
		}
	}()

	dir, err := h.Dir(installPath)
	if err != nil {
		return "", err
	}
	if err := h.SudoFsys().MkDirAll(dir, fs.FileMode(0o755)); err != nil {
		return "", fmt.Errorf("create k0s binary dir %s: %w", dir, err)
	}

	log.Infof("%s: uploading k0s binary from %s to %s", h, src, tmp)
	if err := h.Upload(src, tmp, 0o600, exec.Sudo(h), exec.LogError(true)); err != nil {
		return "", fmt.Errorf("upload k0s binary: %w", err)
	}
	uploaded = true

	stat, err := os.Stat(src)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", src, err)
	}
	if err := h.Touch(tmp, stat.ModTime(), exec.Sudo(h)); err != nil {
		return "", fmt.Errorf("failed to touch %s: %w", tmp, err)
	}
	if !h.IsWindows() {
		if err := h.SetFileMode(tmp, fs.FileMode(0o755)); err != nil {
			log.Warnf("%s: failed to chmod k0s temp binary: %v", h, err)
		}
	}

	return tmp, nil
}

func stageDownload(h Host, url, installPath string, targetVersion *version.Version) (string, error) {
	tmp := stageTempPath(h, installPath)

	dir, err := h.Dir(installPath)
	if err != nil {
		return "", err
	}

	log.Infof("%s: downloading k0s %s", h, targetVersion)
	log.Debugf("%s: ensuring k0s install directory exists %s", h, dir)
	if err := h.SudoFsys().MkDirAll(dir, fs.FileMode(0o755)); err != nil {
		return "", fmt.Errorf("failed to create k0s install directory: %w", err)
	}
	if err := h.DownloadURL(url, tmp, exec.Sudo(h)); err != nil {
		if rmErr := h.DeleteFile(tmp); rmErr != nil {
			log.Debugf("%s: failed to remove partial k0s binary %s: %v", h, tmp, rmErr)
		}
		return "", fmt.Errorf("failed to download k0s binary: %w", err)
	}

	if !h.IsWindows() {
		if err := h.SetFileMode(tmp, fs.FileMode(0o755)); err != nil {
			log.Debugf("%s: chmod %s failed: %v", h, tmp, err)
		}
	}

	return tmp, nil
}
