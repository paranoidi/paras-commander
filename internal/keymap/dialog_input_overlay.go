package keymap

import "strings"

// DefaultDialogInputOverlayKeys holds built-in chords that apply only while a
// dialog input field is focused ([dialog.input]). These act on the
// focused FileDialogField (e.g. restore the suggested default placeholder) and
// are intentionally scoped so the same chord can be reused globally for an
// unrelated action when no dialog input is focused.
func DefaultDialogInputOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionDialogInputRestoreDefault:   {"C-r", "C-d"},
		ActionDialogInputKillWordBackward: {"C-w"},
		ActionDialogInputBackwardWord:     {"M-b"},
		ActionDialogInputForwardWord:      {"M-f"},
	}
}

// AllowedInDialogInputOverlay reports whether actionID may appear under
// [dialog.input]. Only ui.input.* identifiers are accepted so the
// overlay stays scoped to dialog-input editing actions.
func AllowedInDialogInputOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "ui.input.")
}
