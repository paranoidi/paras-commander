package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// previewTerminalKeyOrder are the 4 known scalar string keys PatchPreviewTerminalKeys edits
// inside [preview]. Order controls the order any newly appended keys are written in.
var previewTerminalKeyOrder = []string{
	"terminal_sixel", "terminal_kitty", "terminal_kitty_placeholder", "image_protocol",
}

// PreviewTerminalKeys carries the [preview] values the M-F3 image-capabilities dialog owns.
type PreviewTerminalKeys struct {
	Sixel            string
	Kitty            string
	KittyPlaceholder string
	ImageProtocol    string
}

// previewTableHeaderRe matches a `[preview]` table header line, exact after trimming trailing
// whitespace and an optional inline comment.
var previewTableHeaderRe = regexp.MustCompile(`^\[preview\]\s*(#.*)?$`)

// anyTableHeaderRe matches any TOML table or array-of-tables header line ([section] or
// [[section]]), used to find where the [preview] table's line range ends.
var anyTableHeaderRe = regexp.MustCompile(`^\[.+\]\s*(#.*)?$`)

// previewKeyLineRe returns a regexp matching a line assigning a string value to key
// (e.g. `terminal_sixel = "auto"  # comment`, `terminal_kitty = 'yes'`, or `image_protocol = kitty`),
// capturing the `key = ` prefix (group 1) and any trailing whitespace/comment (group 2) so both can
// be preserved verbatim around the new value. The key must be followed immediately by optional
// whitespace then `=`, so "terminal_kitty" never matches a "terminal_kitty_placeholder" line.
func previewKeyLineRe(key string) *regexp.Regexp {
	return regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(key) + `\s*=\s*)(?:"[^"]*"|'[^']*'|[a-z_]+)(.*)$`)
}

// PatchPreviewTerminalKeysForPaths resolves paths to config.toml and calls PatchPreviewTerminalKeys.
func PatchPreviewTerminalKeysForPaths(paths Paths, keys PreviewTerminalKeys) error {
	configFile, err := resolvePersistPaths(paths)
	if err != nil {
		return err
	}
	return PatchPreviewTerminalKeys(configFile, keys)
}

// PatchPreviewTerminalKeys rewrites exactly the 4 known [preview] scalar keys
// (terminal_sixel, terminal_kitty, terminal_kitty_placeholder, image_protocol) in the TOML file at path, preserving every other line — comments,
// formatting, unrelated tables — untouched. It is intentionally narrow (4 known scalar string
// keys, no general TOML AST editor): the M-F3 image-capabilities dialog is its only caller.
//
// Existing key lines are edited in place, keeping any trailing inline comment. Keys not found
// inside an existing [preview] table are appended at the end of that table. If [preview] itself
// doesn't exist, a new table with all 4 keys is appended at end of file. If the file doesn't
// exist yet, it's created with just a [preview] table holding the 4 keys. The write is atomic
// (temp file + rename, via the same atomicWrite helper WriteMergedPartial uses) so a crash
// mid-write can't truncate the user's config.toml.
func PatchPreviewTerminalKeys(path string, keys PreviewTerminalKeys) error {
	values := map[string]string{
		"terminal_sixel":             keys.Sixel,
		"terminal_kitty":             keys.Kitty,
		"terminal_kitty_placeholder": keys.KittyPlaceholder,
		"image_protocol":             keys.ImageProtocol,
	}

	var lines []string
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		text := strings.TrimSuffix(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
		if text != "" {
			lines = strings.Split(text, "\n")
		}
	case os.IsNotExist(err):
		// Start from an empty file, same as WriteDefaultStub's create-if-missing behavior.
	default:
		return fmt.Errorf("read config %q: %w", path, err)
	}

	tableStart, tableEnd, hasTable := findPreviewTable(lines)

	missing := make(map[string]bool, len(previewTerminalKeyOrder))
	for _, key := range previewTerminalKeyOrder {
		missing[key] = true
	}
	if hasTable {
		for i := tableStart + 1; i < tableEnd; i++ {
			for _, key := range previewTerminalKeyOrder {
				if !missing[key] {
					continue
				}
				re := previewKeyLineRe(key)
				m := re.FindStringSubmatch(lines[i])
				if m == nil {
					continue
				}
				lines[i] = m[1] + `"` + values[key] + `"` + m[2]
				missing[key] = false
			}
		}
	}

	var newKeyLines []string
	for _, key := range previewTerminalKeyOrder {
		if missing[key] {
			newKeyLines = append(newKeyLines, fmt.Sprintf("%s = %q", key, values[key]))
		}
	}

	switch {
	case len(newKeyLines) == 0:
		// All 4 keys already existed and were rewritten in place.
	case hasTable:
		out := make([]string, 0, len(lines)+len(newKeyLines))
		out = append(out, lines[:tableEnd]...)
		out = append(out, newKeyLines...)
		out = append(out, lines[tableEnd:]...)
		lines = out
	default:
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, "[preview]")
		lines = append(lines, newKeyLines...)
	}

	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}
	if err := atomicWrite(path, []byte(out), 0o644); err != nil {
		return fmt.Errorf("write config %q: %w", path, err)
	}
	return nil
}

// findPreviewTable scans lines for a `[preview]` header (exact line match after trim) and
// returns its line index and the exclusive end of its line range (the next `[...]` header, or
// len(lines) if [preview] runs to EOF).
func findPreviewTable(lines []string) (start, end int, ok bool) {
	for i, line := range lines {
		if !previewTableHeaderRe.MatchString(strings.TrimRight(line, " \t")) {
			continue
		}
		end = len(lines)
		for j := i + 1; j < len(lines); j++ {
			if anyTableHeaderRe.MatchString(strings.TrimRight(lines[j], " \t")) {
				end = j
				break
			}
		}
		return i, end, true
	}
	return 0, 0, false
}
