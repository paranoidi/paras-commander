package keymap

// DefaultTerminalOverlayKeys holds built-in chords that apply only while the
// embedded terminal panel is focused ([terminal]). terminal.toggle-panel and
// terminal.focus also have global defaults in DefaultActionSpecs → [main]
// (they must work from the browser even when the panel itself has no focus
// yet); they are repeated here so the same chords keep working once focus is
// inside the panel.
func DefaultTerminalOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionTerminalTogglePanel: {"C-M-p"},
		ActionTerminalFocus:       {"M-p"},
		ActionTerminalGrow:        {"C-k"},
		ActionTerminalShrink:      {"C-j"},
		ActionAppDropToShell:      {"C-o"},
	}
}

// AllowedInTerminalOverlay reports whether actionID may appear under [terminal].
func AllowedInTerminalOverlay(actionID string) bool {
	switch actionID {
	case ActionTerminalTogglePanel, ActionTerminalFocus, ActionTerminalGrow, ActionTerminalShrink, ActionAppDropToShell:
		return true
	default:
		return false
	}
}
