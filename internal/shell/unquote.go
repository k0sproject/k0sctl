package shell

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// This is borrowed from rig v2 until k0sctl is updated to use it

var (
	builderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}

	// ErrMismatchedQuotes is returned when the input string has mismatched quotes when unquoting.
	ErrMismatchedQuotes = errors.New("mismatched quotes")

	// ErrTrailingBackslash is returned when the input string ends with a trailing backslash.
	ErrTrailingBackslash = errors.New("trailing backslash")
)

// Unquote is a mostly POSIX compliant implementation of unquoting a string the same way a shell would.
// Variables and command substitutions are not handled.
func Unquote(input string) (string, error) { //nolint:cyclop
	sb, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		sb = &strings.Builder{}
	}
	defer builderPool.Put(sb)
	defer sb.Reset()

	var inDoubleQuotes, inSingleQuotes, isEscaped bool

	for i := range len(input) {
		currentChar := input[i]

		if isEscaped {
			sb.WriteByte(currentChar)
			isEscaped = false
			continue
		}

		switch currentChar {
		case '\\':
			if !inSingleQuotes { // Escape works in double quotes or outside any quotes
				isEscaped = true
			} else {
				sb.WriteByte(currentChar) // Treat as a regular character within single quotes
			}
		case '"':
			if !inSingleQuotes { // Toggle double quotes only if not in single quotes
				inDoubleQuotes = !inDoubleQuotes
			} else {
				sb.WriteByte(currentChar) // Treat as a regular character within single quotes
			}
		case '\'':
			if !inDoubleQuotes { // Toggle single quotes only if not in double quotes
				inSingleQuotes = !inSingleQuotes
			} else {
				sb.WriteByte(currentChar) // Treat as a regular character within double quotes
			}
		default:
			sb.WriteByte(currentChar)
		}
	}

	if inDoubleQuotes || inSingleQuotes {
		return "", fmt.Errorf("unquote `%q`: %w", input, ErrMismatchedQuotes)
	}

	if isEscaped {
		return "", fmt.Errorf("unquote `%q`: %w", input, ErrTrailingBackslash)
	}

	return sb.String(), nil
}
