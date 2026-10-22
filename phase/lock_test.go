package phase

import (
	"errors"
	"testing"

	"github.com/k0sproject/k0sctl/configurer"
	"github.com/k0sproject/k0sctl/pkg/apis/k0sctl.k0sproject.io/v1beta1/cluster"
	"github.com/k0sproject/rig/exec"
	"github.com/k0sproject/rig/os"
	"github.com/stretchr/testify/require"
)

// lockConfigurer implements only the configurer methods tryLock uses.
type lockConfigurer struct {
	configurer.Configurer
	upsertErr error
	statErr   error
}

func (c *lockConfigurer) K0sctlLockFilePath(os.Host) string {
	return "/run/lock/k0sctl"
}

func (c *lockConfigurer) UpsertFile(os.Host, string, string) error {
	return c.upsertErr
}

func (c *lockConfigurer) Stat(os.Host, string, ...exec.Option) (*os.FileInfo, error) {
	return nil, c.statErr
}

func TestTryLock_ReportsWhyLockFileWasNotCreated(t *testing.T) {
	upsertErr := errors.New("write temp file: Permission denied")
	h := &cluster.Host{Configurer: &lockConfigurer{
		upsertErr: upsertErr,
		statErr:   errors.New("no such file or directory"),
	}}
	p := &Lock{instanceID: "test"}

	err := p.tryLock(h)
	require.ErrorIs(t, err, upsertErr)
	require.ErrorContains(t, err, "failed to create lock file /run/lock/k0sctl")
}
