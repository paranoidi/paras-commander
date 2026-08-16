package keymap

// DefaultRunForEachDialogOverlayKeys holds chords that apply only while the run-for-each
// dialog's main screen is focused (not the command-history picker sub-screen it opens).
// Structurally near-identical to DefaultMassRenameDialogOverlayKeys (mass_rename_dialog_overlay.go)
// — both gate a single "open history picker" action — differing only in which dialog owns it.
func DefaultRunForEachDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFileRunForEachHistory: {"F3"},
	}
}

// AllowedInRunForEachDialogOverlay reports whether actionID may appear under
// [dialog.run_for_each].
func AllowedInRunForEachDialogOverlay(actionID string) bool {
	return actionID == ActionFileRunForEachHistory
}
