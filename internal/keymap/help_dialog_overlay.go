package keymap

// DefaultHelpDialogOverlayKeys holds chords that apply only while the F1 help dialog is open.
func DefaultHelpDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionHelpTextEditKeys: {"F2"},
	}
}

// AllowedInHelpDialogOverlay reports whether actionID may appear under [dialog.help].
func AllowedInHelpDialogOverlay(actionID string) bool {
	return actionID == ActionHelpTextEditKeys
}
