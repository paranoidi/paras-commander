package usermenu

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/panel"
)

// ExpandCommand substitutes % macros in a single command line for argv parsing.
// Recognized: %% %f %F %d %D %t %T (paths are shell-quoted for go-shellwords).
func ExpandCommand(cmd string, active, other *panel.State) (string, error) {
	return ExpandCommandWithFOverride(cmd, active, other, "")
}

// CommandRequiresIteratedF reports whether cmd contains a %f macro (not %%).
func CommandRequiresIteratedF(cmd string) bool {
	return commandContainsMacro(cmd, 'f')
}

func commandContainsMacro(cmd string, letter byte) bool {
	for i := 0; i < len(cmd)-1; i++ {
		if cmd[i] != '%' {
			continue
		}
		if cmd[i+1] == '%' {
			i++
			continue
		}
		if cmd[i+1] == letter {
			return true
		}
	}
	return false
}

// ErrRunForEachRequiresF is returned when a run-for-each command omits %f.
const ErrRunForEachRequiresF = "Command must include %f to represent the selected item"

// ExpandCommandWithFOverride behaves like ExpandCommand, but when fOverride is non-empty,
// %f expands to that value (shell-quoted) instead of the active panel cursor entry.
func ExpandCommandWithFOverride(cmd string, active, other *panel.State, fOverride string) (string, error) {
	if active == nil {
		return "", fmt.Errorf("user menu: no active panel")
	}
	var b strings.Builder
	for i := 0; i < len(cmd); i++ {
		if cmd[i] != '%' || i+1 >= len(cmd) {
			b.WriteByte(cmd[i])
			continue
		}
		switch cmd[i+1] {
		case '%':
			b.WriteByte('%')
			i++
		case 'f':
			if strings.TrimSpace(fOverride) != "" {
				b.WriteString(strconv.Quote(fOverride))
				i++
				continue
			}
			ent, ok := active.CurrentEntry()
			if !ok {
				return "", fmt.Errorf("user menu: %%f: no current file")
			}
			b.WriteString(strconv.Quote(ent.Name))
			i++
		case 'F':
			if other == nil {
				return "", fmt.Errorf("user menu: %%F: no other panel")
			}
			ent, ok := other.CurrentEntry()
			if !ok {
				return "", fmt.Errorf("user menu: %%F: no current file on other panel")
			}
			b.WriteString(strconv.Quote(ent.Name))
			i++
		case 'd':
			b.WriteString(strconv.Quote(filepath.Clean(active.PathString())))
			i++
		case 'D':
			if other == nil {
				return "", fmt.Errorf("user menu: %%D: no other panel")
			}
			b.WriteString(strconv.Quote(filepath.Clean(other.PathString())))
			i++
		case 't':
			s, err := quotedTaggedInDir(active)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
			i++
		case 'T':
			if other == nil {
				return "", fmt.Errorf("user menu: %%T: no other panel")
			}
			s, err := quotedTaggedInDir(other)
			if err != nil {
				return "", err
			}
			b.WriteString(s)
			i++
		default:
			b.WriteByte('%')
		}
	}
	return b.String(), nil
}

func quotedTaggedInDir(ps *panel.State) (string, error) {
	if len(ps.SelectedPaths) == 0 {
		return "", fmt.Errorf("user menu: %%t: no tagged files in panel")
	}
	base := filepath.Clean(ps.PathString())
	var paths []string
	for p := range ps.SelectedPaths {
		if filepath.Clean(filepath.Dir(p)) != base {
			continue
		}
		paths = append(paths, p)
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("user menu: %%t: no tagged files in current directory")
	}
	sort.Strings(paths)
	var b strings.Builder
	for i, p := range paths {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.Quote(p))
	}
	return b.String(), nil
}
