package binprovider

import (
	"io/fs"
	"testing"
	"time"

	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/assert"
)

type fakeHost struct {
	name      string
	isWindows bool
}

func (h *fakeHost) String() string                                  { return h.name }
func (h *fakeHost) FS() remotefs.FS                                 { panic("not used") }
func (h *fakeHost) Sudo() *rig.Client                               { panic("not used") }
func (h *fakeHost) IsWindows() bool                                 { return h.isWindows }
func (h *fakeHost) Arch() (string, error)                           { panic("not used") }
func (h *fakeHost) InstalledK0sVersion() *version.Version           { panic("not used") }
func (h *fakeHost) RunningK0sVersion() *version.Version             { panic("not used") }
func (h *fakeHost) Dir(path string) (string, error)                 { panic("not used") }
func (h *fakeHost) OSKind() (string, error)                         { panic("not used") }
func (h *fakeHost) DownloadURL(url, dest string) error              { panic("not used") }
func (h *fakeHost) Touch(path string, modTime time.Time) error      { panic("not used") }
func (h *fakeHost) DeleteFile(path string) error                    { panic("not used") }
func (h *fakeHost) SetFileMode(path string, mode fs.FileMode) error { panic("not used") }

var _ Host = (*fakeHost)(nil)

func TestNeedsUpgrade(t *testing.T) {
	h := &fakeHost{name: "test-host"}
	v1 := version.MustParse("v1.30.0+k0s.0")
	v2 := version.MustParse("v1.31.0+k0s.0")

	t.Run("no target version", func(t *testing.T) {
		assert.True(t, needsUpgrade(h, nil, v1, v1))
	})

	t.Run("no installed or running version", func(t *testing.T) {
		assert.True(t, needsUpgrade(h, v1, nil, nil))
	})

	t.Run("no running version, installed matches target", func(t *testing.T) {
		assert.False(t, needsUpgrade(h, v1, v1, nil))
	})

	t.Run("no running version, installed differs from target", func(t *testing.T) {
		assert.True(t, needsUpgrade(h, v2, v1, nil))
	})

	t.Run("running version matches target", func(t *testing.T) {
		assert.False(t, needsUpgrade(h, v1, v2, v1))
	})

	t.Run("running version differs from target", func(t *testing.T) {
		assert.True(t, needsUpgrade(h, v2, v2, v1))
	})
}

func TestStageTempPath(t *testing.T) {
	t.Run("linux path gets a plain suffix", func(t *testing.T) {
		h := &fakeHost{}
		got := stageTempPath(h, "/usr/local/bin/k0s")
		assert.True(t, len(got) > len("/usr/local/bin/k0s.tmp."))
		assert.Equal(t, "/usr/local/bin/k0s.tmp.", got[:len("/usr/local/bin/k0s.tmp.")])
	})

	t.Run("windows path preserves the exe extension", func(t *testing.T) {
		h := &fakeHost{isWindows: true}
		got := stageTempPath(h, `C:\k0s\k0s.exe`)
		assert.Equal(t, ".exe", got[len(got)-4:])
		assert.NotEqual(t, `C:\k0s\k0s.exe`, got)
	})

	t.Run("windows path without exe extension gets a plain suffix", func(t *testing.T) {
		h := &fakeHost{isWindows: true}
		got := stageTempPath(h, `C:\k0s\k0s`)
		assert.Contains(t, got, ".tmp.")
	})
}
