package linux

import (
	"fmt"
	"io"
	"io/fs"
	"testing"

	"github.com/k0sproject/rig/exec"
	"github.com/stretchr/testify/require"
)

type mockHost struct {
	ExecFError bool
}

func (m mockHost) Upload(source, destination string, perm fs.FileMode, opts ...exec.Option) error {
	return nil
}

func (m mockHost) Exec(string, ...exec.Option) error {
	return nil
}

func (m mockHost) ExecOutput(string, ...exec.Option) (string, error) {
	return "", nil
}

func (m mockHost) Execf(string, ...any) error {
	if m.ExecFError {
		return fmt.Errorf("error")
	}
	return nil
}

func (m mockHost) ExecOutputf(string, ...any) (string, error) {
	return "", nil
}

func (m mockHost) ExecStreams(cmd string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, opts ...exec.Option) (exec.Waiter, error) {
	return nil, nil
}

func (m mockHost) String() string {
	return ""
}

func (m mockHost) Sudo(string) (string, error) {
	return "", nil
}

func TestPaths(t *testing.T) {
	fc := &Flatcar{}
	fc.SetPath("K0sBinaryPath", "/opt/bin/k0s")

	ubuntu := &Ubuntu{}

	h1 := &mockHost{
		ExecFError: false,
	}
	h2 := &mockHost{
		ExecFError: true,
	}

	require.Equal(t, "/opt/bin/k0s", fc.K0sBinaryPath())
	require.Equal(t, "/usr/local/bin/k0s", ubuntu.K0sBinaryPath())

	require.Equal(t, "/opt/bin/k0s --help", fc.K0sCmdf("--help"))
	require.Equal(t, "/usr/local/bin/k0s --help", ubuntu.K0sCmdf("--help"))

	require.Equal(t, "/var/lib/k0s/pki/admin.conf", fc.KubeconfigPath(h1, fc.DataDirDefaultPath()))
	require.Equal(t, "/var/lib/k0s/pki/admin.conf", ubuntu.KubeconfigPath(h1, ubuntu.DataDirDefaultPath()))

	require.Equal(t, "/var/lib/k0s/kubelet.conf", fc.KubeconfigPath(h2, fc.DataDirDefaultPath()))
	require.Equal(t, "/var/lib/k0s/kubelet.conf", ubuntu.KubeconfigPath(h2, ubuntu.DataDirDefaultPath()))
}
