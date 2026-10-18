package keymap

// DefaultCommandOutputDialogOverlayKeys holds chords that apply only while the user-menu
// command output dialog is open.
func DefaultCommandOutputDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionCommandOutputToggleSelection: {"Insert", "Space"},
	}
}

// AllowedInCommandOutputDialogOverlay reports whether actionID may appear under
// [dialog.command_output].
func AllowedInCommandOutputDialogOverlay(actionID string) bool {
	_, ok := DefaultCommandOutputDialogOverlayKeys()[actionID]
	return ok
}
