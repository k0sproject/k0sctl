package linux

import (
	"bufio"
	"errors"
	"io"
	"testing"

	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/protocol"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/rig/v2/rigtest"
	"github.com/stretchr/testify/require"
)

type mockHost struct {
	ExecOutputValue   string
	ExecOutputErr     error
	LastExecOutputCmd string
	// sudoClient controls the result of Sudo().FS().FileExist(...).
	// v2 KubeconfigPath calls h.Sudo().FS().FileExist(adminConfPath) instead of
	// the old Execf-based approach — use newFileExistClient to construct a client
	// that returns a controlled FileExist result.
	sudoClient *rig.Client
}

func (m *mockHost) String() string  { return "" }
func (m *mockHost) IsWindows() bool { return false }

func (m *mockHost) Exec(string, ...cmd.ExecOption) error {
	return nil
}

func (m *mockHost) ExecOutput(c string, _ ...cmd.ExecOption) (string, error) {
	m.LastExecOutputCmd = c
	if m.ExecOutputErr != nil {
		return "", m.ExecOutputErr
	}
	return m.ExecOutputValue, nil
}

func (m *mockHost) ExecReader(_ string, _ ...cmd.ExecOption) io.Reader       { return nil }
func (m *mockHost) ExecScanner(_ string, _ ...cmd.ExecOption) *bufio.Scanner { return nil }
func (m *mockHost) StartBackground(_ string, _ ...cmd.ExecOption) (protocol.Waiter, error) {
	return nil, nil
}

func (m *mockHost) Sudo() *rig.Client { return m.sudoClient }
func (m *mockHost) FS() remotefs.FS   { return nil }

// newFileExistClient returns a *rig.Client whose FS().FileExist() returns `exists`.
// Used by TestPaths to control which kubeconfig path KubeconfigPath returns.
func newFileExistClient(exists bool) *rig.Client {
	mr := rigtest.NewMockRunner()
	if !exists {
		mr.ErrDefault = errors.New("no such file")
	}
	posixFS := remotefs.NewPosixFS(mr)
	client, err := rig.NewClient(
		rig.WithConnection(mr.MockConnection),
		rig.WithRemoteFSProvider(func(_ cmd.Runner) (remotefs.FS, error) {
			return posixFS, nil
		}),
	)
	if err != nil {
		panic("newFileExistClient: " + err.Error())
	}
	return client
}

func TestPaths(t *testing.T) {
	fc := &Flatcar{}
	fc.SetPath("K0sBinaryPath", "/opt/bin/k0s")

	ubuntu := &Ubuntu{}

	// h1: FileExist returns true  → KubeconfigPath returns adminConf path
	// h2: FileExist returns false → KubeconfigPath falls back to kubelet.conf
	h1 := &mockHost{sudoClient: newFileExistClient(true)}
	h2 := &mockHost{sudoClient: newFileExistClient(false)}

	require.Equal(t, "/opt/bin/k0s", fc.K0sBinaryPath())
	require.Equal(t, "/usr/local/bin/k0s", ubuntu.K0sBinaryPath())

	require.Equal(t, "/opt/bin/k0s --help", fc.K0sCmdf("--help"))
	require.Equal(t, "/usr/local/bin/k0s --help", ubuntu.K0sCmdf("--help"))

	require.Equal(t, "/var/lib/k0s/pki/admin.conf", fc.KubeconfigPath(h1, fc.DataDirDefaultPath()))
	require.Equal(t, "/var/lib/k0s/pki/admin.conf", ubuntu.KubeconfigPath(h1, ubuntu.DataDirDefaultPath()))

	require.Equal(t, "/var/lib/k0s/kubelet.conf", fc.KubeconfigPath(h2, fc.DataDirDefaultPath()))
	require.Equal(t, "/var/lib/k0s/kubelet.conf", ubuntu.KubeconfigPath(h2, ubuntu.DataDirDefaultPath()))
}
