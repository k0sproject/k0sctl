// +build !windows

package cache

import (
	"os"
	"path"
	"runtime"

	"golang.org/x/sys/unix"
)

// Dir returns the directory where k0sctl temporary files should be stored. The directory will be created if it does not exist.
func Dir() string {
	d := defaultdir()
	if EnsureDir(d) == nil && unix.Access(d, unix.W_OK) == nil {
		return d
	}
	return fallbackdir()
}

func defaultdir() string {
	switch runtime.GOOS {
	case "linux":
		return "/var/cache/k0sctl"
	case "darwin":
		home, _ := os.UserHomeDir()
		return path.Join(home, "Library", "Caches", "k0sctl")
	default:
		return path.Join(os.TempDir(), "k0sctl")
	}
}

func fallbackdir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	fb := path.Join(home, ".k0sctl", "cache")
	if err := EnsureDir(fb); err != nil {
		return ""
	}
	return fb
}
