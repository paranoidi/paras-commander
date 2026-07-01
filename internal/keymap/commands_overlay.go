package keymap

import "strings"

// DefaultCommandsOverlayKeys holds built-in chords that apply only while the
// Commands view is focused ([commands]). commands.open defaults live in
// DefaultActionSpecs → [main] (global), same pattern as jobs.open.
func DefaultCommandsOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionCommandsClose:     {"left"},
		ActionCommandsTerminate: {"F8"},
		ActionCommandsKill:      {"S-F8"},
	}
}

// AllowedInCommandsOverlay reports whether actionID may appear under [commands].
func AllowedInCommandsOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "commands.")
}
