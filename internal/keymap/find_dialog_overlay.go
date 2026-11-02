package keymap

// DefaultFindDialogOverlayKeys holds chords that apply only while the find dialog is open.
func DefaultFindDialogOverlayKeys() map[string][]string {
	return withOpenInPanelKeys(map[string][]string{
		ActionFindView:             {"F3"},
		ActionFindEdit:             {"F4"},
		ActionFindUnselectAll:      {"S-F5"},
		ActionFindSelectAll:        {"F5", "C-a"},
		ActionFindSelectGroup:      {"F6"},
		ActionFindUnselectGroup:    {"S-F6"},
		ActionFindSelectParentDirs: {"F2"},
		ActionFindToggleAdvanced:   {"M-a"},
		ActionFindCycleMatchMode:   {"M-m"},
		ActionFindResetAdvanced:    {"M-r"},
	})
}

// AllowedInFindDialogOverlay reports whether actionID may appear under
// [dialog.find].
func AllowedInFindDialogOverlay(actionID string) bool {
	_, ok := DefaultFindDialogOverlayKeys()[actionID]
	return ok
}
