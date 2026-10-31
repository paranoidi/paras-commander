package keymap

import (
	"strings"
	"unicode"
)

// fileIndexMenuGroups is the fileindex menu's grouped layout: one macro column per group, in order.
var fileIndexMenuGroups = []struct {
	title   string
	actions []string
}{
	{"Browse", []string{ActionFileIndexOpen, ActionFileIndexReveal}},
	{"Tagging", []string{ActionFileIndexTagAdd, ActionFileIndexTagRemove}},
	{"Actions", []string{ActionFileIndexScan, ActionFileIndexPrune}},
}

// DefaultFileIndexMenuKeys returns built-in default fileindex-menu letters per action. Each is
// its action's default chord letter in lowercase (every fileindex chord is Alt+Shift+letter).
func DefaultFileIndexMenuKeys() map[string]string {
	out := make(map[string]string)
	for _, spec := range DefaultActionSpecs() {
		if k := strings.TrimSpace(spec.FileIndexMenuKey); k != "" {
			out[spec.ID] = k
		}
	}
	return out
}

func validateFileIndexMenuKeys(keys map[string]string) error {
	return validateMenuKeys(FileIndexMenuShortcutsTable, keys, unicode.IsLetter, "a letter")
}

// FileIndexMenuEntries returns the fileindex menu rows from the bundle's merged keys, grouped
// under a header per column; a group with no bound actions is omitted.
func (b *Bundle) FileIndexMenuEntries() []LeaderMenuEntry {
	if b == nil {
		return nil
	}
	var out []LeaderMenuEntry
	col := 0
	for _, g := range fileIndexMenuGroups {
		var rows []LeaderMenuEntry
		for _, id := range g.actions {
			if key, ok := parseMenuKeyRune(b.FileIndexMenuKey, id); ok {
				rows = append(rows, LeaderMenuEntry{GroupColumn: col, ActionID: id, Key: key, Label: actionSpecTitle(id)})
			}
		}
		if len(rows) == 0 {
			continue
		}
		out = append(out, LeaderMenuEntry{GroupTitle: g.title, GroupColumn: col})
		out = append(out, rows...)
		col++
	}
	return out
}
