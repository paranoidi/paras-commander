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
	ActionClipboardGitMenu,
}

// copyMenuGitActionOrder is the "Relative to git root" submenu; its keys are
// unique within the submenu only, so they may repeat top-level keys.
var copyMenuGitActionOrder = []string{
	ActionClipboardCopyGitFilePath,
	ActionClipboardCopyGitDirPath,
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
	scopes := [][]string{copyMenuActionOrder, copyMenuGitActionOrder}
	for _, order := range scopes {
		scoped := make(map[string]string)
		for _, id := range order {
			if k, ok := keys[id]; ok {
				scoped[id] = k
			}
		}
		if err := validateMenuKeys("copy_menu", scoped, validCopyMenuKeyRune, "a letter"); err != nil {
			return err
		}
	}
	return nil
}

func copyMenuEntryForAction(keys map[string]string, actionID string) (CopyMenuEntry, bool) {
	key, ok := parseMenuKeyRune(keys, actionID)
	if !ok {
		return CopyMenuEntry{}, false
	}
	label := actionSpecTitle(actionID)
	if spec, ok := SpecForAction(actionID); ok && spec.CopyMenuLabel != "" {
		label = spec.CopyMenuLabel
	}
	return CopyMenuEntry{
		ActionID: actionID,
		Key:      key,
		Label:    label,
	}, true
}

// BuildCopyMenuEntries returns copy-menu rows in display order.
func BuildCopyMenuEntries(keys map[string]string) []CopyMenuEntry {
	return buildCopyMenuEntries(keys, copyMenuActionOrder)
}

func buildCopyMenuEntries(keys map[string]string, order []string) []CopyMenuEntry {
	if len(keys) == 0 {
		return nil
	}
	var out []CopyMenuEntry
	for _, actionID := range order {
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

// CopyMenuGitEntries returns the "Relative to git root" submenu rows.
func (b *Bundle) CopyMenuGitEntries() []CopyMenuEntry {
	if b == nil {
		return nil
	}
	return buildCopyMenuEntries(b.CopyMenuKey, copyMenuGitActionOrder)
}
