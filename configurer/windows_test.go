package configurer

import (
	"testing"

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
