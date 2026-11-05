package keymap

// DefaultMetaDialogOverlayKeys holds chords that apply only while the meta columns picker is open.
func DefaultMetaDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPanelMetaSave: {"F5"},
	}
}

// AllowedInMetaDialogOverlay reports whether actionID may appear under
// [dialog.meta].
func AllowedInMetaDialogOverlay(actionID string) bool {
	_, ok := DefaultMetaDialogOverlayKeys()[actionID]
	return ok
}
