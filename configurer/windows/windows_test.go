package windows

import (
	"bufio"
	"fmt"
	"io"
	"testing"

	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	ps "github.com/k0sproject/rig/v2/powershell"
	"github.com/k0sproject/rig/v2/protocol"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/stretchr/testify/require"
)


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

func (h *stubHost) String() string    { return "stub" }
func (h *stubHost) IsWindows() bool   { return false }

func (h *stubHost) Exec(string, ...cmd.ExecOption) error {
	return nil
}

func (h *stubHost) ExecOutput(c string, _ ...cmd.ExecOption) (string, error) {
	h.execCalls = append(h.execCalls, c)
	resp, ok := h.execOutputs[c]
	if !ok {
		return "", fmt.Errorf("unexpected command: %s", c)
	}
	return resp.output, resp.err
}

func (h *stubHost) ExecReader(_ string, _ ...cmd.ExecOption) io.Reader       { return nil }
func (h *stubHost) ExecScanner(_ string, _ ...cmd.ExecOption) *bufio.Scanner { return nil }
func (h *stubHost) StartBackground(_ string, _ ...cmd.ExecOption) (protocol.Waiter, error) {
	return nil, nil
}

func (h *stubHost) Sudo() *rig.Client  { return nil }
func (h *stubHost) FS() remotefs.FS    { return nil }
