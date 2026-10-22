package configurer

import (
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"

	"github.com/k0sproject/rig/exec"
	"github.com/stretchr/testify/require"
)

type execCall struct {
	cmd  string
	sudo bool
}

// recordingHost records the commands run through Execf and whether they were
// run with sudo. Commands starting with "test -f" fail so that UpsertFile sees
// the temp file as moved into place.
type recordingHost struct {
	calls []execCall
}

func (r *recordingHost) Upload(string, string, fs.FileMode, ...exec.Option) error {
	return nil
}

func (r *recordingHost) Exec(string, ...exec.Option) error {
	return nil
}

func (r *recordingHost) ExecOutput(cmd string, _ ...exec.Option) (string, error) {
	if cmd == "mktemp" {
		return "/tmp/tmp.abc", nil
	}
	return "", nil
}

func (r *recordingHost) Execf(cmd string, argsOrOpts ...any) error {
	var args []any
	var opts []exec.Option
	for _, a := range argsOrOpts {
		if o, ok := a.(exec.Option); ok {
			opts = append(opts, o)
			continue
		}
		args = append(args, a)
	}
	full := fmt.Sprintf(cmd, args...)
	r.calls = append(r.calls, execCall{cmd: full, sudo: exec.Build(opts...).Sudo})
	if strings.HasPrefix(full, "test -f") {
		return fmt.Errorf("not found")
	}
	return nil
}

func (r *recordingHost) ExecOutputf(cmd string, args ...any) (string, error) {
	return r.ExecOutput(fmt.Sprintf(cmd, args...))
}

func (r *recordingHost) ExecStreams(string, io.ReadCloser, io.Writer, io.Writer, ...exec.Option) (exec.Waiter, error) {
	return nil, nil
}

func (r *recordingHost) String() string {
	return "recording"
}

func (r *recordingHost) Sudo(cmd string) (string, error) {
	return cmd, nil
}

func TestUpsertFile_WritesTempFileAsConnectingUser(t *testing.T) {
	h := &recordingHost{}
	l := &Linux{}

	require.NoError(t, l.UpsertFile(h, "/run/lock/k0sctl", "instance"))

	var write, move *execCall
	for i, c := range h.calls {
		switch {
		case strings.HasPrefix(c.cmd, "cat >"):
			write = &h.calls[i]
		case strings.HasPrefix(c.cmd, "mv -n"):
			move = &h.calls[i]
		}
	}
	require.NotNil(t, write, "temp file was not written")
	require.NotNil(t, move, "temp file was not moved into place")

	// mktemp creates the file as the connecting user, writing it as root is
	// denied in /tmp when fs.protected_regular=2
	require.Equal(t, `cat > "/tmp/tmp.abc"`, write.cmd)
	require.False(t, write.sudo, "temp file must be written without sudo")

	require.Equal(t, `mv -n "/tmp/tmp.abc" "/run/lock/k0sctl"`, move.cmd)
	require.True(t, move.sudo, "temp file must be moved into place with sudo")
}
