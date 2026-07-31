package platform

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// appleScriptStringLiteral renders path as a double-quoted AppleScript string
// literal, including the surrounding quotes.
//
// The escaping ORDER matters and must not be swapped: backslashes are doubled
// first, quotes second. Escaping quotes first would introduce backslashes that
// the backslash pass would then double, corrupting the literal.
//
// AppleScript string literals cannot contain raw control characters. A file
// name holding one (a newline is the realistic case on macOS, where almost
// anything except "/" and NUL is a legal name) used to reach osascript and come
// back as an opaque parse error, so such paths are rejected here with a message
// that says what is actually wrong.
func appleScriptStringLiteral(path string) (string, error) {
	if path == "" {
		return "", errors.New("empty path")
	}
	if !utf8.ValidString(path) {
		return "", fmt.Errorf("path %q is not valid UTF-8 and cannot be passed to osascript", path)
	}

	for i, r := range path {
		switch {
		case r == '\n' || r == '\r':
			return "", fmt.Errorf(
				"path contains a line break at byte %d and cannot be passed to osascript; "+
					"rename the item or delete it permanently instead", i)
		case r < 0x20 || r == 0x7f:
			return "", fmt.Errorf(
				"path contains the control character %U at byte %d and cannot be passed to osascript; "+
					"rename the item or delete it permanently instead", r, i)
		}
	}

	// Backslashes first, then quotes. Do not reorder.
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `"` + escaped + `"`, nil
}
