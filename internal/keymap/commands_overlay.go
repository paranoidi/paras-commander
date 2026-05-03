package keymap

import "strings"

// DefaultCommandsOverlayKeys holds built-in chords that apply only while the
// Commands view is focused ([commands_action_keys]). commands.open defaults live in
// DefaultActionSpecs → [action_keys] (global), same pattern as jobs.open.
func DefaultCommandsOverlayKeys() map[string][]string {
	return map[string][]string{}
}

// AllowedInCommandsOverlay reports whether actionID may appear under [commands_action_keys].
func AllowedInCommandsOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "commands.")
}
