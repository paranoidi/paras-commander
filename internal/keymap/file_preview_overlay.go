package keymap

import "strings"

// DefaultFilePreviewOverlayKeys holds built-in chords that apply only while the
// full-screen preview (F3) is focused ([preview]).
func DefaultFilePreviewOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPreviewMenu:         {":"},
		ActionPreviewThemePicker:  {"F9"},
		ActionPreviewToggleRaw:    {"F6"},
		ActionPreviewReload:       {"F5"},
		ActionPreviewSearchStart:  {"/"},
		ActionPreviewSearchNext:   {"n"},
		ActionPreviewSearchPrev:   {"p"},
		ActionPreviewClose:        {"q", "h"},
		ActionPreviewScrollDown:   {"j"},
		ActionPreviewScrollUp:     {"k"},
		ActionPreviewHalfPageDown: {"C-d"},
		ActionPreviewHalfPageUp:   {"C-u"},
		ActionPreviewPageDown:     {"C-f"},
		ActionPreviewPageUp:       {"C-b"},
		ActionPreviewTop:          {"g"},
		ActionPreviewBottom:       {"G"},
	}
}

// AllowedInFilePreviewOverlay reports whether actionID may appear under [preview].
func AllowedInFilePreviewOverlay(actionID string) bool {
	if _, ok := KnownActions[actionID]; !ok {
		return false
	}
	return strings.HasPrefix(actionID, "preview.") && actionID != ActionPreviewSettingsDialog
}

// DefaultFilePreviewSearchOverlayKeys holds built-in chords for [preview_search]: match-jump
// chords consulted only while a fullscreen-preview search has matches. They are a separate table
// from [preview] because that overlay is consulted before the global map and would shadow the
// plain scroll/page chords when no search is active.
func DefaultFilePreviewSearchOverlayKeys() map[string][]string {
	return map[string][]string{
		ActionPreviewSearchNext: {"down", "C-j"},
		ActionPreviewSearchPrev: {"up", "C-k"},
	}
}

// AllowedInFilePreviewSearchOverlay reports whether actionID may appear under [preview_search].
func AllowedInFilePreviewSearchOverlay(actionID string) bool {
	_, ok := DefaultFilePreviewSearchOverlayKeys()[actionID]
	return ok
}
