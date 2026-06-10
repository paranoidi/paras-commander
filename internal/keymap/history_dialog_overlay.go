package keymap

// DefaultHistoryDialogOverlayKeys holds chords that apply only while the history dialog is open.
func DefaultHistoryDialogOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPanelHistoryBothPanels: {"F5"},
	}
}

// AllowedInHistoryDialogOverlay reports whether actionID may appear under
// [history_dialog_action_keys].
func AllowedInHistoryDialogOverlay(actionID string) bool {
	switch actionID {
	case ActionPanelHistoryBothPanels:
		return true
	default:
		return false
	}
}
