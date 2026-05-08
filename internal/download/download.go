package download

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

var httpClient = &http.Client{
	Timeout: 10 * time.Minute,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	},
}

// ToFile downloads url to dest using a temporary file in the destination directory.
func ToFile(ctx context.Context, url, dest string) (retErr error) {
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpFile, err := os.CreateTemp(dir, filepath.Base(dest)+".tmp-")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if tmpFile != nil {
			if err := tmpFile.Close(); err != nil && retErr == nil {
				retErr = err
			}
		}
		if retErr != nil {
			if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
				log.Warnf("failed to remove partial download at %s: %v", tmpPath, err)
			}
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status %s from %s", resp.Status, url)
	}
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		return err
	}
	if err := tmpFile.Close(); err != nil {
		tmpFile = nil
		return err
	}
	tmpFile = nil
	// os.Rename is atomic on Unix (replaces dest if it exists), so concurrent runs are safe.
	// On Windows it fails if dest already exists; two simultaneous k0sctl processes targeting
	// the same destination could race here. We intentionally propagate that error rather than
	// silently accepting whatever file is at dest, which would be a TOCTOU risk.
	if err := os.Rename(tmpPath, dest); err != nil {
		return err
	}
	return nil
}
