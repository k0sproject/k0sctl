package configurer

import (
	"bufio"
	"fmt"
	"io"
	"testing"

	rig "github.com/k0sproject/rig/v2"
	"github.com/k0sproject/rig/v2/cmd"
	"github.com/k0sproject/rig/v2/protocol"
	"github.com/k0sproject/rig/v2/remotefs"
	"github.com/stretchr/testify/require"
)

func TestBaseWindowsQuote(t *testing.T) {
	w := &BaseWindows{}
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "safe string",
			input:  "10.0.0.148",
			expect: "10.0.0.148",
		},
		{
			name:   "safe path",
			input:  `C:\Users\Admin\var\lib\k0s`,
			expect: `C:\Users\Admin\var\lib\k0s`,
		},
		{
			name:   "needs quotes for space",
			input:  `C:\Program Files\k0s`,
			expect: `"C:\Program Files\k0s"`,
		},
		{
			name:   "needs quotes for special char",
			input:  `value with spaces & symbols`,
			expect: `"value with spaces & symbols"`,
		},
		{
			name:   "empty string",
			input:  "",
			expect: `""`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expect, w.Quote(tt.input))
		})
	}
}

func TestBaseWindowsLookPath(t *testing.T) {
	w := &BaseWindows{}
	mh := &mockWindowsHost{execOutputValue: "C:\\Program Files\\k0s\\k0s.exe\r\n"}

	path, err := w.LookPath(mh, "k0s")
	require.NoError(t, err)
	require.Equal(t, "C:/Program Files/k0s/k0s.exe", path)
}

func TestBaseWindowsLookPathNotFound(t *testing.T) {
	w := &BaseWindows{}
	mh := &mockWindowsHost{execOutputErr: fmt.Errorf("exit status 1")}

	path, err := w.LookPath(mh, "missing")
	require.Error(t, err)
	require.Empty(t, path)
}

func TestBaseWindowsLookPathEmpty(t *testing.T) {
	w := &BaseWindows{}
	mh := &mockWindowsHost{}

	path, err := w.LookPath(mh, " ")
	require.Error(t, err)
	require.Empty(t, path)
	require.Empty(t, mh.lastExecOutputCmd)
}

type mockWindowsHost struct {
	execOutputValue   string
	execOutputErr     error
	lastExecOutputCmd string
}

func (m *mockWindowsHost) String() string { return "" }
func (m *mockWindowsHost) IsWindows() bool { return false }

func (m *mockWindowsHost) Exec(string, ...cmd.ExecOption) error {
	return nil
}

func (m *mockWindowsHost) ExecOutput(c string, _ ...cmd.ExecOption) (string, error) {
	m.lastExecOutputCmd = c
	if m.execOutputErr != nil {
		return "", m.execOutputErr
	}
	return m.execOutputValue, nil
}

func (m *mockWindowsHost) ExecReader(_ string, _ ...cmd.ExecOption) io.Reader        { return nil }
func (m *mockWindowsHost) ExecScanner(_ string, _ ...cmd.ExecOption) *bufio.Scanner  { return nil }
func (m *mockWindowsHost) StartBackground(_ string, _ ...cmd.ExecOption) (protocol.Waiter, error) {
	return nil, nil
}

func (m *mockWindowsHost) Sudo() *rig.Client   { return nil }
func (m *mockWindowsHost) FS() remotefs.FS     { return nil }
