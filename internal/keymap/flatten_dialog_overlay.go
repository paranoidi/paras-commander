package keymap

// DefaultFlattenDialogOverlayKeys holds chords that apply only while the flatten
// dialog destination row is focused.
func DefaultFlattenDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionDestinationActivePanel:   {"S-left"},
		ActionDestinationInactivePanel: {"S-right"},
	}
}

// AllowedInFlattenDialogOverlay reports whether actionID may appear under
// [dialog.flatten].
func AllowedInFlattenDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionDestinationActivePanel, ActionDestinationInactivePanel:
		return true
	default:
		return false
	}
}
