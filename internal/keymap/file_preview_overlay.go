package keymap

import "strings"

// DefaultFilePreviewOverlayKeys holds built-in chords that apply only while the
// full-screen file view (F3) is focused ([file_preview]).
func DefaultFilePreviewOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionFileViewMenu:        {":"},
		ActionFileViewThemePicker: {"F9"},
		ActionFileViewToggleRaw:   {"F6"},
		ActionFileViewReload:      {"F5"},
		ActionFileViewSearchStart: {"/"},
		ActionFileViewSearchNext:  {"n"},
		ActionFileViewSearchPrev:  {"p"},
		ActionFileViewClose:       {"q"},
	}
}

// AllowedInFilePreviewOverlay reports whether actionID may appear under [file_preview].
func AllowedInFilePreviewOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "file.view.")
}
