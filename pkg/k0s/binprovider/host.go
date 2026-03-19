package binprovider

import (
	"fmt"
	"io/fs"
	"time"

	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/pkg/rigfs"
	"github.com/k0sproject/version"
)

// Host defines the subset of host behavior that the binary providers rely on.
// cluster.Host implements this interface via thin wrappers over its configurer
// and metadata fields so the providers can live in this standalone package.
type Host interface {
	fmt.Stringer

	// Basic rig/exec capabilities used by exec.Sudo and file transfers.
	Sudo(cmd string) (string, error)
	Upload(src, dst string, perm fs.FileMode, opts ...exec.Option) error
	SudoFsys() rigfs.Fsys
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
	DownloadURL(url, dest string, opts ...exec.Option) error
	Touch(path string, modTime time.Time, opts ...exec.Option) error
	DeleteFile(path string) error
	SetFileMode(path string, mode fs.FileMode) error
}
