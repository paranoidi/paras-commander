package ui

import "github.com/paranoidi/paras-commander/internal/ui/dialog"

// PinnedItem is one ad-hoc, session-only pinned file or directory. Pins are
// never persisted — the list resets on every app restart.
type PinnedItem struct {
	Path  string
	IsDir bool
	// PathMissing is recomputed each time the Pin dialog opens (not live per-render)
	// and drives the same red dialog.option.invalid styling as the bookmarks/history
	// pickers use for a path that no longer exists on disk.
	PathMissing bool
}

// pinDialogItems projects Model.PinnedItems into the dialog package's own row type for
// DrawPinDialog. internal/ui/dialog cannot import internal/ui (ui already imports dialog),
// so this render-time conversion — not a duplicated copy of the pin list itself — is how the
// dialog stays a thin view over Model.PinnedItems, the single source of truth.
func pinDialogItems(items []PinnedItem) []dialog.PinDialogItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]dialog.PinDialogItem, len(items))
	for i, it := range items {
		out[i] = dialog.PinDialogItem{Path: it.Path, IsDir: it.IsDir, PathMissing: it.PathMissing}
	}
	return out
}

// PinnedPathSet builds an O(1) lookup set from PinnedItems for per-row rendering checks.
// Callers rebuild this each render pass rather than caching it — PinnedItems is small
// (user-curated) and changes only on explicit keypresses, so the rebuild cost is negligible
// and this avoids a second piece of state that could drift from PinnedItems.
func PinnedPathSet(items []PinnedItem) map[string]struct{} {
	if len(items) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(items))
	for _, it := range items {
		set[it.Path] = struct{}{}
	}
	return set
}

// otherPanelMatchNames returns, for the inactive panel panelID, the entry names of the active
// panel's directory and its cursor entry name; nil when the mark is off, panelID is the active
// panel, or both panels show the same directory.
// ponytail: rebuilt per render (O(n) over the active listing); cache on State keyed by listing generation if huge dirs lag.
func otherPanelMatchNames(model Model, panelID int) (map[string]struct{}, string) {
	if !model.OtherPanelMatchMarks || panelID == model.ActivePanel {
		return nil, ""
	}
	active, inactive := &model.Primary, &model.Secondary
	if model.ActivePanel == SecondaryPanel {
		active, inactive = inactive, active
	}
	if active.PathString() == inactive.PathString() || len(active.Entries) == 0 {
		return nil, ""
	}
	names := make(map[string]struct{}, len(active.Entries))
	for _, e := range active.Entries {
		names[e.Name] = struct{}{}
	}
	cursor := ""
	if e, ok := active.CurrentEntry(); ok && active.EntryDir(e).String() == active.Path.String() {
		cursor = e.Name
	}
	return names, cursor
}
