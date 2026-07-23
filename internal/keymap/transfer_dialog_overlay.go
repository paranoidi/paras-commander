package keymap

// DefaultTransferDialogOverlayKeys holds chords that apply only while the copy/move
// (transfer) dialog destination row is focused.
func DefaultTransferDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionDestinationActivePanel:   {"S-left"},
		ActionDestinationInactivePanel: {"S-right"},
	}
}

// AllowedInTransferDialogOverlay reports whether actionID may appear under
// [dialog.transfer].
func AllowedInTransferDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionDestinationActivePanel, ActionDestinationInactivePanel:
		return true
	default:
		return false
	}
}
