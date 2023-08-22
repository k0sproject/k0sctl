package linux

import (
	"testing"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/stretchr/testify/require"
)

// TestPaths tests the slightly weird way to perform function overloading
func TestPaths(t *testing.T) {
	fc := &Flatcar{}
	fc.PathFuncs = interface{}(fc).(configurer.PathFuncs)

	ubuntu := &Ubuntu{}
	ubuntu.PathFuncs = interface{}(ubuntu).(configurer.PathFuncs)

	require.Equal(t, "/opt/bin/k0s", fc.K0sBinaryPath())
	require.Equal(t, "/usr/local/bin/k0s", ubuntu.K0sBinaryPath())
}
