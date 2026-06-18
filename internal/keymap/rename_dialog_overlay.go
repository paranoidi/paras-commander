package keymap

// DefaultRenameDialogOverlayKeys holds chords that apply only while the main
// rename dialog (name field) is focused — not during sanitize/slugify sub-screens.
func DefaultRenameDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFileRenameOpenSanitize: {"F2"},
		ActionFileRenameOpenSlugify:  {"F3"},
		ActionFileRenameOpenEncoding: {"F4"},
	}
}

// AllowedInRenameDialogOverlay reports whether actionID may appear under
// [dialog.rename].
func AllowedInRenameDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFileRenameOpenSanitize, ActionFileRenameOpenSlugify, ActionFileRenameOpenEncoding:
		return true
	default:
		return false
	}
}
