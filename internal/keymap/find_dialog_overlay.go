package keymap

// DefaultFindDialogOverlayKeys holds chords that apply only while the find dialog is open.
func DefaultFindDialogOverlayKeys() map[string][]string {
	return withOpenInPanelKeys(map[string][]string{
		ActionFindView:             {"F3"},
		ActionFindUnselectAll:      {"F4"},
		ActionFindSelectAll:        {"F5", "M-a"},
		ActionFindSelectGroup:      {"F6"},
		ActionFindUnselectGroup:    {"F7"},
		ActionFindSelectParentDirs: {"F2"},
	})
}

// AllowedInFindDialogOverlay reports whether actionID may appear under
// [dialog.find].
func AllowedInFindDialogOverlay(actionID string) bool {
	_, ok := DefaultFindDialogOverlayKeys()[actionID]
	return ok
}
