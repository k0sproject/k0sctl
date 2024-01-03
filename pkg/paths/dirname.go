// Package paths contains utility functions for handling file paths independently from the local OS unlike the path/filepath package.
package paths

import (
	"regexp"
	"strings"
)

var multipleSlashesRegex = regexp.MustCompile("/{2,}")

// Dir is like filepath.Dir but it doesn't use the local OS path separator.
func Dir(path string) string {
	// Normalize path by replacing multiple slashes with a single slash
	normalizedPath := multipleSlashesRegex.ReplaceAllString(path, "/")

	// Your existing logic here
	if normalizedPath == "/" {
		return "/"
	}
	if strings.HasSuffix(normalizedPath, "/") {
		return strings.TrimSuffix(normalizedPath, "/")
	}
	idx := strings.LastIndex(normalizedPath, "/")
	switch idx {
	case 0:
		return "/"
	case -1:
		return "."
	default:
		return normalizedPath[:idx]
	}
}
