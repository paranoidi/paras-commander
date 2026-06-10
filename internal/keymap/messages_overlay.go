package keymap

import "strings"

// DefaultMessagesOverlayKeys holds built-in chords that apply only while the
// Messages view is focused ([messages]). messages.open defaults live in
// DefaultActionSpecs → [main] (global), same pattern as jobs.open / commands.open.
func DefaultMessagesOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionMessagesClose: {"left"},
		ActionMessagesClear: {"F8"},
	}
}

// AllowedInMessagesOverlay reports whether actionID may appear under [messages].
func AllowedInMessagesOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "messages.")
}
