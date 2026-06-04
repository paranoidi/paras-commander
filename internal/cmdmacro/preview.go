package cmdmacro

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	maxTerminalWidth = 4096
)

// ClampTerminalWidth returns width clamped to 1..4096.
func ClampTerminalWidth(width int) int {
	if width < 1 {
		return 1
	}
	if width > maxTerminalWidth {
		return maxTerminalWidth
	}
	return width
}

// ReplaceTerminalWidth substitutes every %w with the clamped width string.
func ReplaceTerminalWidth(line string, terminalWidth int) string {
	return strings.ReplaceAll(line, "%w", strconv.Itoa(ClampTerminalWidth(terminalWidth)))
}

// CountPathMacro counts %f placeholders (not %%).
func CountPathMacro(line string) int {
	n := 0
	for i := 0; i < len(line)-1; i++ {
		if line[i] != '%' {
			continue
		}
		if line[i+1] == '%' {
			i++
			continue
		}
		if line[i+1] == 'f' {
			n++
		}
	}
	return n
}

// LegacyPreviewPlaceholders reports an error if template uses deprecated {path} or {terminal_width}.
func LegacyPreviewPlaceholders(line string) error {
	if strings.Contains(line, "{path}") || strings.Contains(line, "{terminal_width}") {
		return fmt.Errorf("preview command: use %%f and %%w instead of {path} and {terminal_width}")
	}
	return nil
}
