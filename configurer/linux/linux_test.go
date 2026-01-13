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
	ExecFError        bool
	ExecOutputValue   string
	ExecOutputErr     error
	LastExecOutputCmd string
}

func (m *mockHost) Upload(source, destination string, perm fs.FileMode, opts ...exec.Option) error {
	return nil
}

func (m *mockHost) Exec(string, ...exec.Option) error {
	return nil
}

func (m *mockHost) ExecOutput(cmd string, opts ...exec.Option) (string, error) {
	m.LastExecOutputCmd = cmd
	if m.ExecOutputErr != nil {
		return "", m.ExecOutputErr
	}
	return m.ExecOutputValue, nil
}

func (m *mockHost) Execf(string, ...any) error {
	if m.ExecFError {
		return fmt.Errorf("error")
	}
	return nil
}

func (m *mockHost) ExecOutputf(cmd string, args ...any) (string, error) {
	return m.ExecOutput(fmt.Sprintf(cmd, args...))
}

func (m *mockHost) ExecStreams(cmd string, stdin io.ReadCloser, stdout io.Writer, stderr io.Writer, opts ...exec.Option) (exec.Waiter, error) {
	return nil, nil
}

func (m *mockHost) String() string {
	return ""
}

func (m *mockHost) Sudo(string) (string, error) {
	return "", nil
}

func TestPaths(t *testing.T) {
	fc := &Flatcar{}
	fc.SetPath("K0sBinaryPath", "/opt/bin/k0s")

	ubuntu := &Ubuntu{}

	h1 := &mockHost{}
	h2 := &mockHost{ExecFError: true}

	require.Equal(t, "/opt/bin/k0s", fc.K0sBinaryPath())
	require.Equal(t, "/usr/local/bin/k0s", ubuntu.K0sBinaryPath())

	require.Equal(t, "/opt/bin/k0s --help", fc.K0sCmdf("--help"))
	require.Equal(t, "/usr/local/bin/k0s --help", ubuntu.K0sCmdf("--help"))

	require.Equal(t, "/var/lib/k0s/pki/admin.conf", fc.KubeconfigPath(h1, fc.DataDirDefaultPath()))
	require.Equal(t, "/var/lib/k0s/pki/admin.conf", ubuntu.KubeconfigPath(h1, ubuntu.DataDirDefaultPath()))

	require.Equal(t, "/var/lib/k0s/kubelet.conf", fc.KubeconfigPath(h2, fc.DataDirDefaultPath()))
	require.Equal(t, "/var/lib/k0s/kubelet.conf", ubuntu.KubeconfigPath(h2, ubuntu.DataDirDefaultPath()))
}

func TestLookPath(t *testing.T) {
	linuxCfg := &Ubuntu{}
	mh := &mockHost{ExecOutputValue: "/usr/bin/k0s\n"}
	path, err := linuxCfg.LookPath(mh, "k0s")
	require.NoError(t, err)
	require.Equal(t, "/usr/bin/k0s", path)
	require.Contains(t, mh.LastExecOutputCmd, "command -v -- k0s")
}

func TestLookPathNotFound(t *testing.T) {
	linuxCfg := &Ubuntu{}
	mh := &mockHost{ExecOutputErr: fmt.Errorf("exit status 1")}
	path, err := linuxCfg.LookPath(mh, "missing")
	require.Error(t, err)
	require.Empty(t, path)
}
