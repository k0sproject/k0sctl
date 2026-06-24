package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
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

// ToFile downloads rawURL to dest using a temporary file in the destination directory.
func ToFile(ctx context.Context, rawURL, dest string) (retErr error) {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create download request for %s: %w", RedactedURL(rawURL), redactedURLError(rawURL, err))
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", RedactedURL(rawURL), redactedURLError(rawURL, err))
	}
	defer func() {
		if err := resp.Body.Close(); err != nil && retErr == nil {
			retErr = err
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status %s from %s", resp.Status, RedactedURL(rawURL))
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

// RedactedURL returns a URL string suitable for error messages.
func RedactedURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "<redacted>"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	return parsed.String()
}

func redactedURLError(rawURL string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		return urlErr.Err
	}
	return errors.New(strings.ReplaceAll(err.Error(), rawURL, RedactedURL(rawURL)))
}
