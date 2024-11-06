package shell

// this is borrowed as-is from rig v2 until k0sctl is updated to use it

import (
	"fmt"
	"strings"
)

// Split splits the input string respecting shell-like quoted segments.
func Split(input string) ([]string, error) { //nolint:cyclop
	var segments []string

	currentSegment, ok := builderPool.Get().(*strings.Builder)
	if !ok {
		currentSegment = &strings.Builder{}
	}
	defer builderPool.Put(currentSegment)
	defer currentSegment.Reset()

	var inDoubleQuotes, inSingleQuotes, isEscaped bool

	for i := range len(input) {
		currentChar := input[i]

		if isEscaped {
			currentSegment.WriteByte(currentChar)
			isEscaped = false
			continue
		}

		switch {
		case currentChar == '\\' && !inSingleQuotes:
			isEscaped = true
		case currentChar == '"' && !inSingleQuotes:
			inDoubleQuotes = !inDoubleQuotes
		case currentChar == '\'' && !inDoubleQuotes:
			inSingleQuotes = !inSingleQuotes
		case currentChar == ' ' && !inDoubleQuotes && !inSingleQuotes:
			// Space outside quotes; delimiter for a new segment
			segments = append(segments, currentSegment.String())
			currentSegment.Reset()
		default:
			currentSegment.WriteByte(currentChar)
		}
	}

	if inDoubleQuotes || inSingleQuotes {
		return nil, fmt.Errorf("split `%q`: %w", input, ErrMismatchedQuotes)
	}

	if isEscaped {
		return nil, fmt.Errorf("split `%q`: %w", input, ErrTrailingBackslash)
	}

	// Add the last segment if present
	if currentSegment.Len() > 0 {
		segments = append(segments, currentSegment.String())
	}

	return segments, nil
}
