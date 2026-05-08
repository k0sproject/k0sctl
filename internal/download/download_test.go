package download

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToFileDownloadsToDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, err := fmt.Fprint(w, "downloaded")
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	dest := filepath.Join(t.TempDir(), "bundle")
	require.NoError(t, ToFile(context.Background(), server.URL+"/bundle", dest))

	content, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "downloaded", string(content))
	require.Empty(t, tempFiles(t, dest))
}

func TestToFileRedactsURLOnHTTPStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	dest := filepath.Join(t.TempDir(), "bundle")
	err := ToFile(context.Background(), authenticatedURL(server.URL)+"/bundle?token=secret", dest)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unexpected http status 401 Unauthorized")
	require.NotContains(t, err.Error(), "token=secret")
	require.NotContains(t, err.Error(), "user:pass")
	require.NoFileExists(t, dest)
	require.Empty(t, tempFiles(t, dest))
}

func TestToFileRemovesPartialDownloadOnCopyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "10")
		_, err := fmt.Fprint(w, "part")
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	dest := filepath.Join(t.TempDir(), "bundle")
	err := ToFile(context.Background(), server.URL+"/bundle", dest)
	require.Error(t, err)
	require.NoFileExists(t, dest)
	require.Empty(t, tempFiles(t, dest))
}

func TestToFileRemovesTempFileOnCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dest := filepath.Join(t.TempDir(), "bundle")
	err := ToFile(ctx, "http://127.0.0.1/bundle", dest)
	require.Error(t, err)
	require.NoFileExists(t, dest)
	require.Empty(t, tempFiles(t, dest))
}

func TestRedactedURLRemovesCredentialsAndQuery(t *testing.T) {
	got := RedactedURL("https://user:pass@example.invalid/path/to/bundle?token=secret#fragment")
	require.Equal(t, "https://example.invalid/path/to/bundle", got)
}

func authenticatedURL(rawURL string) string {
	return strings.Replace(rawURL, "http://", "http://user:pass@", 1)
}

func tempFiles(t *testing.T, dest string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(dest), filepath.Base(dest)+".tmp-*"))
	require.NoError(t, err)
	return matches
}
