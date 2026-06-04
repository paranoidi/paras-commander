package cmdrun

import (
	"fmt"
	"strings"

	"github.com/paranoidi/paras-commander/internal/cmdmacro"
)

// PreviewCommandArgv builds argv for file preview from a single-line command template.
// Use %w for terminal width and %f for the absolute file path (at most one %f).
// If %f is absent, the path is appended as the final argument after parsing.
func PreviewCommandArgv(commandLine, absPath string, terminalWidth int) ([]string, error) {
	line := strings.TrimSpace(commandLine)
	if err := cmdmacro.LegacyPreviewPlaceholders(line); err != nil {
		return nil, err
	}
	line = cmdmacro.ReplaceTerminalWidth(line, terminalWidth)
	if line == "" {
		return nil, fmt.Errorf("empty preview command")
	}
	n := cmdmacro.CountPathMacro(line)
	if n > 1 {
		return nil, fmt.Errorf("preview command: at most one %%f")
	}
	if n == 1 {
		idx := indexPathMacro(line)
		if idx < 0 {
			return nil, fmt.Errorf("preview command: %%f not found")
		}
		left := strings.TrimSpace(line[:idx])
		right := strings.TrimSpace(line[idx+2:])
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

func indexPathMacro(line string) int {
	for i := 0; i < len(line)-1; i++ {
		if line[i] != '%' {
			continue
		}
		if line[i+1] == '%' {
			i++
			continue
		}
		if line[i+1] == 'f' {
			return i
		}
	}
	return -1
}
