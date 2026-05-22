package keymap

import "strings"

// DefaultMessagesOverlayKeys holds built-in chords that apply only while the
// Messages view is focused ([messages_action_keys]). messages.open defaults live in
// DefaultActionSpecs → [action_keys] (global), same pattern as jobs.open / commands.open.
func DefaultMessagesOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionMessagesClear: {"F8"},
	}
}

// AllowedInMessagesOverlay reports whether actionID may appear under [messages_action_keys].
func AllowedInMessagesOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "messages.")
}
