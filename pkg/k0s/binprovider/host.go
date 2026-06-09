package binprovider

import (
	"fmt"
	"io/fs"
	"time"

	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/version"
)

// Host defines the subset of host behavior that the binary providers rely on.
// cluster.Host implements this interface via its embedded rig client and thin
// wrappers over its configurer and metadata fields so the providers can live in
// this standalone package.
type Host interface {
	fmt.Stringer

	// Sudo returns a privilege-escalated rig client used for file transfers and
	// command execution that require elevated permissions.
	Sudo() *rig.Client
	IsWindows() bool

	// Host facts.
	Arch() (string, error)

	// Cached k0s version metadata gathered during facts collection.
	InstalledK0sVersion() *version.Version
	RunningK0sVersion() *version.Version

	// Configurer-backed helpers with graceful error reporting when the
	// configurer has not been resolved yet.
	Dir(path string) (string, error)
	OSKind() (string, error)
	DownloadURL(url, dest string) error
	Touch(path string, modTime time.Time) error
	DeleteFile(path string) error
	SetFileMode(path string, mode fs.FileMode) error
}
