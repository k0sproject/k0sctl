package linux

import (
	"fmt"
	"testing"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/rig/exec"
	"github.com/stretchr/testify/require"
)

type mockHost struct {
	ExecFError bool
}

func (m mockHost) Upload(source, destination string, opts ...exec.Option) error {
	return nil
}

func (m mockHost) Exec(string, ...exec.Option) error {
	return nil
}

func (m mockHost) ExecOutput(string, ...exec.Option) (string, error) {
	return "", nil
}

func (m mockHost) Execf(string, ...interface{}) error {
	if m.ExecFError {
		return fmt.Errorf("error")
	}
	return nil
}

func (m mockHost) ExecOutputf(string, ...interface{}) (string, error) {
	return "", nil
}

func (m mockHost) String() string {
	return ""
}

func (m mockHost) Sudo(string) (string, error) {
	return "", nil
}

// TestPaths tests the slightly weird way to perform function overloading
func TestPaths(t *testing.T) {
	fc := &Flatcar{}
	fc.PathFuncs = interface{}(fc).(configurer.PathFuncs)

	ubuntu := &Ubuntu{}
	ubuntu.PathFuncs = interface{}(ubuntu).(configurer.PathFuncs)

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

	require.Equal(t, "/var/lib/k0s/pki/admin.conf", fc.KubeconfigPath(h1))
	require.Equal(t, "/var/lib/k0s/pki/admin.conf", ubuntu.KubeconfigPath(h1))

	require.Equal(t, "/var/lib/k0s/kubelet.conf", fc.KubeconfigPath(h2))
	require.Equal(t, "/var/lib/k0s/kubelet.conf", ubuntu.KubeconfigPath(h2))
}
