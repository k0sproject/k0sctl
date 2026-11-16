package phase

import (
	"errors"
	"testing"

	"github.com/k0sproject/k0sctl/configurer/linux"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/k0sproject/rig/v2/rigtest"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
)

func TestUpgradeControllers_ValidateStagedBinaries(t *testing.T) {
	const staged = "/tmp/k0s.staged"

	// newHost returns a controller running v1.30 with a binary from a custom k0sBinaryPath staged on it.
	// The staged binary reports stagedVersion.
	newHost := func(t *testing.T, stagedVersion string) *cluster.Host {
		t.Helper()
		mr := rigtest.NewMockRunner()
		mr.ErrDefault = errors.New("unexpected command")
		mr.AddCommand(rigtest.Match("sudo.*true"), func(_ *rigtest.A) error { return nil })
		mr.AddCommandOutput(rigtest.Contains(staged+" version"), stagedVersion+"\n")
		posixFS := remotefs.NewPosixFS(mr)
		client, err := rig.NewClient(
			rig.WithConnection(mr.MockConnection),
			rig.WithRemoteFSProvider(func(_ cmd.Runner) (remotefs.FS, error) {
				return posixFS, nil
			}),
		)
		require.NoError(t, err)
		return &cluster.Host{
			Client:        client,
			Configurer:    &linux.Ubuntu{},
			Role:          "controller",
			K0sBinaryPath: "/local/path/to/k0s",
			Metadata: cluster.HostMetadata{
				K0sRunningVersion: version.MustParse("v1.30.5+k0s.0"),
				K0sBinaryTempFile: staged,
				NeedsUpgrade:      true,
			},
		}
	}

	t.Run("blocks a staged binary that skips a minor version", func(t *testing.T) {
		h := newHost(t, "v1.32.0+k0s.0")
		installed := h.K0sInstallLocation()
		p := &UpgradeControllers{hosts: cluster.Hosts{h}}
		err := p.validateStagedBinaries()
		require.ErrorContains(t, err, "from v1.30.5+k0s.0 to v1.32.0+k0s.0")
		require.ErrorContains(t, err, "one minor version at a time")
		require.Equal(t, installed, h.K0sInstallLocation(), "the k0s binary path must be restored")
	})

	t.Run("allows a staged binary one minor version ahead", func(t *testing.T) {
		p := &UpgradeControllers{hosts: cluster.Hosts{newHost(t, "v1.31.1+k0s.0")}}
		require.NoError(t, p.validateStagedBinaries())
	})

	t.Run("can be skipped", func(t *testing.T) {
		p := &UpgradeControllers{hosts: cluster.Hosts{newHost(t, "v1.32.0+k0s.0")}, SkipVersionSkewCheck: true}
		require.NoError(t, p.validateStagedBinaries())
	})

	t.Run("does not run the staged binary of a host without a custom k0sBinaryPath", func(t *testing.T) {
		// The version of a downloaded binary is verified against the configured version when it is installed
		h := newHost(t, "not a version")
		h.K0sBinaryPath = ""
		p := &UpgradeControllers{hosts: cluster.Hosts{h}}
		require.NoError(t, p.validateStagedBinaries())
	})
}
