package windows

import (
	"fmt"
	"io"
	"io/fs"
	"testing"

	"github.com/k0sproject/rig/exec"
	ps "github.com/k0sproject/rig/pkg/powershell"
	"github.com/stretchr/testify/require"
)

func TestWriteFileScriptUsesUTF8Encoding(t *testing.T) {
	script := writeFileScript("C:/etc/k0s/k0stoken")
	require.Contains(t, script, `[System.Text.UTF8Encoding]::new($false)`)
	require.Contains(t, script, `C:\etc\k0s\k0stoken`)
}

func TestValidateHostRequiresContainersFeature(t *testing.T) {
	featureCmd := ps.Cmd(`(Get-WindowsFeature -Name Containers -ErrorAction SilentlyContinue).InstallState`)
	optionalCmd := ps.Cmd(`(Get-WindowsOptionalFeature -Online -FeatureName Containers -ErrorAction SilentlyContinue).State`)
	w := &Windows{}

	t.Run("feature already installed", func(t *testing.T) {
		h := newStubHost(map[string]commandResponse{
			featureCmd: {output: "Installed"},
		})
		require.NoError(t, w.ValidateHost(h))
		require.Equal(t, []string{featureCmd}, h.execCalls)
	})

	t.Run("falls back to optional feature", func(t *testing.T) {
		h := newStubHost(map[string]commandResponse{
			featureCmd:  {err: fmt.Errorf("not available")},
			optionalCmd: {output: "Enabled"},
		})
		require.NoError(t, w.ValidateHost(h))
		require.Equal(t, []string{featureCmd, optionalCmd}, h.execCalls)
	})

	t.Run("fails when feature disabled", func(t *testing.T) {
		h := newStubHost(map[string]commandResponse{
			featureCmd: {output: "Available"},
		})
		err := w.ValidateHost(h)
		require.ErrorContains(t, err, `must be enabled (current state: Available)`)
	})

	t.Run("fails when state cannot be detected", func(t *testing.T) {
		h := newStubHost(map[string]commandResponse{
			featureCmd:  {err: fmt.Errorf("missing cmd")},
			optionalCmd: {err: fmt.Errorf("also missing")},
		})
		err := w.ValidateHost(h)
		require.ErrorContains(t, err, "failed to detect Containers feature state")
	})
}

type commandResponse struct {
	output string
	err    error
}

type stubHost struct {
	execOutputs map[string]commandResponse
	execCalls   []string
}

func newStubHost(outputs map[string]commandResponse) *stubHost {
	return &stubHost{execOutputs: outputs}
}

func (h *stubHost) Upload(string, string, fs.FileMode, ...exec.Option) error {
	return nil
}

func (h *stubHost) Exec(string, ...exec.Option) error {
	return nil
}

func (h *stubHost) ExecOutput(cmd string, _ ...exec.Option) (string, error) {
	h.execCalls = append(h.execCalls, cmd)
	resp, ok := h.execOutputs[cmd]
	if !ok {
		return "", fmt.Errorf("unexpected command: %s", cmd)
	}
	return resp.output, resp.err
}

func (h *stubHost) Execf(string, ...interface{}) error {
	return nil
}

func (h *stubHost) ExecOutputf(string, ...interface{}) (string, error) {
	return "", nil
}

func (h *stubHost) ExecStreams(string, io.ReadCloser, io.Writer, io.Writer, ...exec.Option) (exec.Waiter, error) {
	return nil, nil
}

func (h *stubHost) String() string {
	return "stub"
}

func (h *stubHost) Sudo(cmd string) (string, error) {
	return cmd, nil
}
