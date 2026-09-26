package keymap

import "strings"

// DefaultFilePreviewOverlayKeys holds built-in chords that apply only while the
// full-screen preview (F3) is focused ([preview]).
func DefaultFilePreviewOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPreviewMenu:        {":"},
		ActionPreviewThemePicker: {"F9"},
		ActionPreviewToggleRaw:   {"F6"},
		ActionPreviewReload:      {"F5"},
		ActionPreviewSearchStart: {"/"},
		ActionPreviewSearchNext:  {"n"},
		ActionPreviewSearchPrev:  {"p"},
		ActionPreviewClose:       {"q"},
	}
}

// AllowedInFilePreviewOverlay reports whether actionID may appear under [preview].
func AllowedInFilePreviewOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "preview.") && actionID != ActionPreviewSettingsDialog
}
