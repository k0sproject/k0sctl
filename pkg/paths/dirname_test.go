package paths

import (
	"testing"
)

func TestDir(t *testing.T) {
	testCases := []struct {
		path     string
		expected string
	}{
		{"/usr/local/bin", "/usr/local"},
		{"/usr/local/bin/", "/usr/local/bin"},
		{"usr/local/bin/", "usr/local/bin"},
		{"usr/local/bin", "usr/local"},
		{"/usr", "/"},
		{"usr", "."},
		{"/", "/"},
		{"", "."},
		{"filename.txt", "."},
		{"./file.txt", "."},
		{"../sibling.txt", ".."},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := Dir(tc.path)
			if result != tc.expected {
				t.Errorf("Dir(%q) = %q, want %q", tc.path, result, tc.expected)
			}
		})
	}
}
