package keymap

import (
	"strings"
	"unicode"
)

var copyMenuActionOrder = []string{
	ActionClipboardCopyFileURL,
	ActionClipboardCopyDirURL,
	ActionClipboardCopyFilename,
	ActionClipboardCopyFilenameWithoutExt,
}

// CopyMenuEntry is one row in the `"` copy menu.
type CopyMenuEntry struct {
	ActionID string
	Key      rune
	Label    string
}

// DefaultCopyMenuKeys returns built-in default copy-menu letters per action.
func DefaultCopyMenuKeys() map[string]string {
	out := make(map[string]string)
	for _, spec := range DefaultActionSpecs() {
		if k := strings.TrimSpace(spec.CopyMenuKey); k != "" {
			out[spec.ID] = k
		}
	}
	return out
}

func mergeCopyMenuKeys(defaults, user map[string]string) map[string]string {
	return mergeMenuKeys(defaults, user)
}

func validCopyMenuKeyRune(r rune) bool {
	return unicode.IsLetter(r)
}

func validateCopyMenuKeys(keys map[string]string) error {
	return validateMenuKeys("copy_menu", keys, validCopyMenuKeyRune, "a letter")
}

func copyMenuEntryForAction(keys map[string]string, actionID string) (CopyMenuEntry, bool) {
	key, ok := parseMenuKeyRune(keys, actionID)
	if !ok {
		return CopyMenuEntry{}, false
	}
	return CopyMenuEntry{
		ActionID: actionID,
		Key:      key,
		Label:    actionSpecTitle(actionID),
	}, true
}

// BuildCopyMenuEntries returns copy-menu rows in display order.
func BuildCopyMenuEntries(keys map[string]string) []CopyMenuEntry {
	if len(keys) == 0 {
		return nil
	}
	var out []CopyMenuEntry
	for _, actionID := range copyMenuActionOrder {
		if ent, ok := copyMenuEntryForAction(keys, actionID); ok {
			out = append(out, ent)
		}
	}
	return out
}

// CopyMenuEntries returns copy-menu rows from the bundle's merged keys.
func (b *Bundle) CopyMenuEntries() []CopyMenuEntry {
	if b == nil || len(b.CopyMenuKey) == 0 {
		return nil
	}
	return BuildCopyMenuEntries(b.CopyMenuKey)
}
