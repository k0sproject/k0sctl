// +build !windows

package cache

import (
	"os"
	"path"
	"runtime"
)

// Dir returns the directory where k0sctl temporary files should be stored. The directory will be created if it does not exist.
func Dir() string {
	d := defaultdir()
	if err := EnsureDir(d); err != nil {
		// Fall back to ~/.k0sctl/cache
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

	return d
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
