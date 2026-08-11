package keymap

// DefaultFindDialogOverlayKeys holds chords that apply only while the find dialog is open.
func DefaultFindDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFindView:            {"F3"},
		ActionFindUnselectAll:     {"F4"},
		ActionFindSelectAll:       {"F5", "C-a"},
		ActionFindSelectGroup:     {"F6"},
		ActionFindUnselectGroup:   {"F7"},
		ActionFindOpenInPrimary:   {"S-left"},
		ActionFindOpenInSecondary: {"S-right"},
	}
}

// AllowedInFindDialogOverlay reports whether actionID may appear under
// [dialog.find].
func AllowedInFindDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFindView, ActionFindSelectAll, ActionFindUnselectAll, ActionFindSelectGroup, ActionFindUnselectGroup,
		ActionFindOpenInPrimary, ActionFindOpenInSecondary:
		return true
	default:
		return false
	}
}
