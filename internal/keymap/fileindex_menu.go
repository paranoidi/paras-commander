package keymap

import (
	"strings"
	"unicode"
)

// fileIndexMenuActionOrder is the fileindex menu's row order.
var fileIndexMenuActionOrder = []string{
	ActionFileIndexOpen,
	ActionFileIndexReveal,
	ActionFileIndexScan,
	ActionFileIndexPrune,
	ActionFileIndexTagAdd,
	ActionFileIndexTagRemove,
}

// FileIndexMenuEntry is one row in the fileindex menu (ActionFileIndexMenu).
type FileIndexMenuEntry struct {
	ActionID string
	Key      rune
	Label    string
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

// FileIndexMenuEntries returns the fileindex menu rows from the bundle's merged keys.
func (b *Bundle) FileIndexMenuEntries() []FileIndexMenuEntry {
	if b == nil {
		return nil
	}
	var out []FileIndexMenuEntry
	for _, id := range fileIndexMenuActionOrder {
		if key, ok := parseMenuKeyRune(b.FileIndexMenuKey, id); ok {
			out = append(out, FileIndexMenuEntry{ActionID: id, Key: key, Label: actionSpecTitle(id)})
		}
	}
	return out
}
