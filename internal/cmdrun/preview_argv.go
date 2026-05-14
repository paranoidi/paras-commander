package cmdrun

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	previewPathPlaceholder            = "{path}"
	previewTerminalWidthPlaceholder = "{terminal_width}"
)

// PreviewCommandArgv builds argv for file preview from a single-line command template.
// Every previewTerminalWidthPlaceholder is replaced with terminalWidth (clamped to 1..4096) so
// highlighters can match the inactive column (e.g. bat --terminal-width).
// If the template contains previewPathPlaceholder exactly once, the absolute path is inserted
// as one argv token between the left and right sides (each side parsed with ParseCommandArgv).
// If the placeholder is absent, ParseCommandArgv is applied to the whole line and absPath is
// appended as the final argument. Multiple path placeholders return an error.
func PreviewCommandArgv(commandLine, absPath string, terminalWidth int) ([]string, error) {
	if terminalWidth < 1 {
		terminalWidth = 1
	}
	if terminalWidth > 4096 {
		terminalWidth = 4096
	}
	line := strings.TrimSpace(commandLine)
	line = strings.ReplaceAll(line, previewTerminalWidthPlaceholder, strconv.Itoa(terminalWidth))
	if line == "" {
		return nil, fmt.Errorf("empty preview command")
	}
	n := strings.Count(line, previewPathPlaceholder)
	if n > 1 {
		return nil, fmt.Errorf("preview command: at most one %s placeholder", previewPathPlaceholder)
	}
	if n == 1 {
		idx := strings.Index(line, previewPathPlaceholder)
		left := strings.TrimSpace(line[:idx])
		right := strings.TrimSpace(line[idx+len(previewPathPlaceholder):])
		var out []string
		if left != "" {
			a, err := ParseCommandArgv(left)
			if err != nil {
				return nil, err
			}
			out = append(out, a...)
		}
		out = append(out, absPath)
		if right != "" {
			a, err := ParseCommandArgv(right)
			if err != nil {
				return nil, err
			}
			out = append(out, a...)
		}
		if len(out) == 0 {
			return []string{absPath}, nil
		}
		return out, nil
	}
	argv, err := ParseCommandArgv(line)
	if err != nil {
		return nil, err
	}
	return append(argv, absPath), nil
}
