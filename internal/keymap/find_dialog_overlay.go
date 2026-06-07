package keymap

// DefaultFindDialogOverlayKeys holds chords that apply only while the find dialog is open.
func DefaultFindDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFindSelectAll:     {"F5", "C-a"},
		ActionFindSelectGroup:   {"F6"},
		ActionFindUnselectGroup: {"F8"},
	}
}

// AllowedInFindDialogOverlay reports whether actionID may appear under
// [find_dialog_action_keys].
func AllowedInFindDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFindSelectAll, ActionFindSelectGroup, ActionFindUnselectGroup:
		return true
	default:
		return false
	}
}
