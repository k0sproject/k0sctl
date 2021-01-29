// +build !windows
package cache

import (
	"os"
	"path"
	"runtime"
)

// Directory where k0sctl temporary files should be stored
func Dir() string {
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
