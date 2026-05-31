package keymap

// DefaultFindDialogOverlayKeys holds chords that apply only while the find dialog is open.
func DefaultFindDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFindSelectAll: {"F5", "C-a"},
	}
}

// AllowedInFindDialogOverlay reports whether actionID may appear under
// [find_dialog_action_keys].
func AllowedInFindDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFindSelectAll:
		return true
	default:
		return false
	}
}
