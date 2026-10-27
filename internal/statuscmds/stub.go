package statuscmds

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/paranoidi/paras-commander/internal/configdoc"
)

// StatusStubTOML is written when no status.toml exists yet. Entries are commented so the
// user can uncomment and customize.
const StatusStubTOML = `# status.toml — menu-bar status commands
#
# Each shown command runs on its own interval; the first line of its output is drawn
# at the top-left of the menu bar. Segments are separated by the theme's
# icons.menu.status.separator glyph (style menu.status.separator). Empty output, a
# non-zero exit or a run over 10 s hides the segment.
#
# show  [string]  names of the commands to draw, in order ([] = none). The
#                 Display → Status commands dialog rewrites this line.
#
# Built-in commands: loadavg, memory, battery.
#
# [[command]] fields (a built-in name replaces that built-in):
#   name         string  required, unique
#   label        string  dialog checkbox text (default: name)
#   command      string  required; run via sh -c, no %f-style macros
#   interval_ms  int     default 3000, values below 500 use the default
#   max_width    int     default 15, clamped to [1, 200]; longer output is ellipsized
#   color        string  text color: a theme [palette] name (e.g. "yellow"), "#RRGGBB"
#                        or "default"; empty uses the theme's menu.status style
#
# [[command]]
# name = "clock"
# label = "Clock"
# command = "date +%H:%M"
# interval_ms = 10000
# max_width = 5
# color = "bright_yellow"
#
# --- end of documentation ---
`

// WriteStub creates path with StatusStubTOML when the file does not exist yet.
func WriteStub(path string) (created bool, err error) {
	if strings.TrimSpace(path) == "" {
		return false, fmt.Errorf("status stub path is required")
	}
	path = filepath.Clean(path)
	if _, statErr := os.Stat(path); statErr == nil {
		return false, nil
	} else if !os.IsNotExist(statErr) {
		return false, fmt.Errorf("stat status stub %q: %w", path, statErr)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("create status stub dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(StatusStubTOML), 0o644); err != nil {
		return false, fmt.Errorf("write status stub %q: %w", path, err)
	}
	return true, nil
}

// SaveShow sets the top-level show key in path to names, creating the file from the stub
// when missing. Everything else in the file is kept byte-for-byte.
func SaveShow(path string, names []string) error {
	if _, err := WriteStub(path); err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := configdoc.AtomicWrite(path, []byte(replaceShow(string(b), showLine(names))), 0o644); err != nil {
		return fmt.Errorf("write %q: %w", path, err)
	}
	return nil
}

func showLine(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = strconv.Quote(n)
	}
	return "show = [" + strings.Join(quoted, ", ") + "]"
}

// replaceShow swaps the top-level show assignment (possibly spanning lines) in content for
// line. Without one, line goes right after the documentation sentinel, or first.
func replaceShow(content, line string) string {
	lines := strings.Split(content, "\n")
	insertAt := 0
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == configdoc.DocEndSentinel {
			insertAt = i + 1
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			break // first table: show must be above it
		}
		if !isShowAssignment(trimmed) {
			continue
		}
		end := i
		if strings.Contains(trimmed, "[") && !strings.Contains(trimmed, "]") {
			for end+1 < len(lines) && !strings.Contains(lines[end], "]") {
				end++
			}
		}
		out := append([]string{}, lines[:i]...)
		out = append(out, line)
		out = append(out, lines[end+1:]...)
		return strings.Join(out, "\n")
	}
	out := append([]string{}, lines[:insertAt]...)
	out = append(out, line)
	out = append(out, lines[insertAt:]...)
	return strings.Join(out, "\n")
}

func isShowAssignment(trimmed string) bool {
	rest, ok := strings.CutPrefix(trimmed, "show")
	return ok && strings.HasPrefix(strings.TrimSpace(rest), "=")
}
