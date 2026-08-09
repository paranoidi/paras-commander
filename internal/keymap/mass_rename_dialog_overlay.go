package keymap

// DefaultMassRenameDialogOverlayKeys holds chords that apply only while the mass-rename
// dialog's main find/replace screen is focused — not during the save-pattern prompt or
// load-pattern/history picker sub-screens.
func DefaultMassRenameDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFileMassRenameLoadPattern:   {"F2"},
		ActionFileMassRenameHistory:       {"F3"},
		ActionFileMassRenameSavePattern:   {"F5"},
		ActionFileMassRenameDeletePattern: {"F8"}, // mirrors bookmarks' F8 "Delete bookmark"
	}
}

// AllowedInMassRenameDialogOverlay reports whether actionID may appear under
// [dialog.mass_rename].
func AllowedInMassRenameDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionFileMassRenameSavePattern, ActionFileMassRenameLoadPattern, ActionFileMassRenameHistory, ActionFileMassRenameDeletePattern:
		return true
	default:
		return false
	}
}
