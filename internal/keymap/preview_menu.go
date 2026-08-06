package keymap

import (
	"strings"
	"unicode"
)

var previewMenuActionOrder = []string{
	ActionFileViewThemePicker,
	ActionFileViewToggleRaw,
	ActionFileViewReload,
	ActionFileViewSearchStart,
	ActionFileViewDiffNextHunk,
	ActionFileViewDiffPrevHunk,
	ActionFileEdit,
	ActionFileDelete,
	ActionAppQuit,
}

// PreviewMenuEntry is one row in the `:` fullscreen-preview menu.
type PreviewMenuEntry struct {
	ActionID string
	Key      rune
	Label    string
}

// DefaultPreviewMenuKeys returns built-in default preview-menu letters per action.
func DefaultPreviewMenuKeys() map[string]string {
	out := make(map[string]string)
	for _, spec := range DefaultActionSpecs() {
		if k := strings.TrimSpace(spec.PreviewMenuKey); k != "" {
			out[spec.ID] = k
		}
	}
	return out
}

func mergePreviewMenuKeys(defaults, user map[string]string) map[string]string {
	return mergeMenuKeys(defaults, user)
}

func validPreviewMenuKeyRune(r rune) bool {
	return unicode.IsLetter(r)
}

func validatePreviewMenuKeys(keys map[string]string) error {
	return validateMenuKeys("preview_menu", keys, validPreviewMenuKeyRune, "a letter")
}

func previewMenuEntryForAction(keys map[string]string, actionID string) (PreviewMenuEntry, bool) {
	key, ok := parseMenuKeyRune(keys, actionID)
	if !ok {
		return PreviewMenuEntry{}, false
	}
	return PreviewMenuEntry{
		ActionID: actionID,
		Key:      key,
		Label:    actionSpecTitle(actionID),
	}, true
}

// BuildPreviewMenuEntries returns preview-menu rows in display order.
func BuildPreviewMenuEntries(keys map[string]string) []PreviewMenuEntry {
	if len(keys) == 0 {
		return nil
	}
	var out []PreviewMenuEntry
	for _, actionID := range previewMenuActionOrder {
		if ent, ok := previewMenuEntryForAction(keys, actionID); ok {
			out = append(out, ent)
		}
	}
	return out
}

// PreviewMenuEntries returns preview-menu rows from the bundle's merged keys.
func (b *Bundle) PreviewMenuEntries() []PreviewMenuEntry {
	if b == nil || len(b.PreviewMenuKey) == 0 {
		return nil
	}
	return BuildPreviewMenuEntries(b.PreviewMenuKey)
}
