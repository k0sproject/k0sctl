package cluster

import (
	"testing"

	"github.com/k0sproject/rig/v2/powershell"
	"github.com/stretchr/testify/require"
)

// posixQuoter (declared in flags_test.go) mirrors the ShellQuote of a POSIX
// remotefs: simple tokens are left bare, only values that need it are quoted.

// winQuoter mirrors remotefs.WinFS.ShellQuote, which is powershell.SingleQuote
// and therefore *always* wraps its argument in single quotes.
type winQuoter struct{}

func (winQuoter) ShellQuote(s string) string { return powershell.SingleQuote(s) }

// buildInstallFlags reproduces the flag construction that Host.K0sInstallFlags
// performs for a worker (data-dir/token-file/kubelet-root-dir via the host
// quoter, plus a --kubelet-extra-args built from a nested Flags), and returns
// the final command tail as K0sInstallCommand would render it via Join.
func buildInstallFlags(q quoter, dataDir, tokenFile, kubeletRootDir string) string {
	var flags Flags
	flags.AddOrReplace("--data-dir=" + quote(q, dataDir))
	flags.AddUnlessExist("--token-file=" + quote(q, tokenFile))
	flags.AddOrReplace("--kubelet-root-dir=" + quote(q, kubeletRootDir))

	var extra Flags
	extra.AddOrReplace("--cluster-dns=10.96.0.10")
	extra.AddOrReplace("--hostname-override=EC2AMAZ-XXXX")
	// K0sInstallFlags builds --kubelet-extra-args from extra.Join(q); the outer
	// quote that used to wrap this has been removed (see issue #1114).
	flags.AddOrReplace("--kubelet-extra-args=" + extra.Join(q))

	return flags.Join(q)
}

// TestK0sInstallFlagsQuoting is the table form of the standalone repro from
// issue #1114. It runs against both a POSIX quoter and a Windows SingleQuote
// quoter so the Windows-specific double-unquote (backslash paths) and
// double-quote (--kubelet-extra-args) regressions are guarded.
func TestK0sInstallFlagsQuoting(t *testing.T) {
	tests := []struct {
		name           string
		quoter         quoter
		dataDir        string
		tokenFile      string
		kubeletRootDir string
		expected       string
	}{
		{
			name:           "posix",
			quoter:         posixQuoter{},
			dataDir:        "/var/lib/k0s",
			tokenFile:      "/etc/k0s/k0stoken",
			kubeletRootDir: "/var/lib/k0s/kubelet",
			expected:       `--data-dir=/var/lib/k0s --token-file=/etc/k0s/k0stoken --kubelet-root-dir=/var/lib/k0s/kubelet --kubelet-extra-args='--cluster-dns=10.96.0.10 --hostname-override=EC2AMAZ-XXXX'`,
		},
		{
			name:           "windows",
			quoter:         winQuoter{},
			dataDir:        powershell.ToWindowsPath("C:/var/lib/k0s"),
			tokenFile:      powershell.ToWindowsPath("C:/etc/k0s/k0stoken"),
			kubeletRootDir: powershell.ToWindowsPath("C:/var/lib/k0s/kubelet"),
			expected:       `--data-dir='C:\var\lib\k0s' --token-file='C:\etc\k0s\k0stoken' --kubelet-root-dir='C:\var\lib\k0s\kubelet' --kubelet-extra-args='--cluster-dns=10.96.0.10 --hostname-override=EC2AMAZ-XXXX'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildInstallFlags(tt.quoter, tt.dataDir, tt.tokenFile, tt.kubeletRootDir)
			require.Equal(t, tt.expected, got)
		})
	}
}

// TestFlagsUnquoteOnce guards the primitive: a value quoted once by the host
// quoter must be stored unquoted exactly once, with any literal backslashes
// intact. Before the fix, AddOrReplace/AddUnlessExist unquoted a second time
// via Add, and Each unquoted bare values, so Windows backslash paths were
// silently corrupted (C:\var\lib\k0s -> C:varlibk0s).
func TestFlagsUnquoteOnce(t *testing.T) {
	quoters := []struct {
		name string
		q    quoter
	}{
		{"posix", posixQuoter{}},
		{"windows", winQuoter{}},
	}

	for _, qc := range quoters {
		t.Run(qc.name, func(t *testing.T) {
			const winPath = `C:\var\lib\k0s`

			t.Run("AddOrReplace preserves backslashes", func(t *testing.T) {
				var f Flags
				f.AddOrReplace("--data-dir=" + quote(qc.q, winPath))
				require.Equal(t, []string{`--data-dir=` + winPath}, []string(f))
			})

			t.Run("AddUnlessExist preserves backslashes", func(t *testing.T) {
				var f Flags
				f.AddUnlessExist("--data-dir=" + quote(qc.q, winPath))
				require.Equal(t, []string{`--data-dir=` + winPath}, []string(f))
			})

			t.Run("Each leaves a bare value untouched", func(t *testing.T) {
				f := Flags{`--data-dir=` + winPath}
				f.Each(func(k, v string) {
					require.Equal(t, "--data-dir", k)
					require.Equal(t, winPath, v)
				})
			})

			t.Run("Each still unquotes a genuinely quoted value", func(t *testing.T) {
				f := Flags{`--foo='bar baz'`}
				f.Each(func(k, v string) {
					require.Equal(t, "--foo", k)
					require.Equal(t, "bar baz", v)
				})
			})
		})
	}
}
