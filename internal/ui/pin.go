package ui

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

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

// PinDialogItems projects Model.PinnedItems into the dialog package's own row type for
// DrawPinDialog. internal/ui/dialog cannot import internal/ui (ui already imports dialog),
// so this render-time conversion — not a duplicated copy of the pin list itself — is how the
// dialog stays a thin view over Model.PinnedItems, the single source of truth.
func PinDialogItems(items []PinnedItem) []dialog.PinDialogItem {
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

// otherMatchDir is the directory a panel compares against the other panel: in tree layout the
// cursor row's folder when that row is an expanded directory, else the cursor row's parent;
// outside tree layout, the panel path.
func otherMatchDir(s *panel.State) string {
	if e, ok := s.CurrentEntry(); ok && e.Type == localfs.EntryDirectory && e.Name != ".." {
		if _, _, _, expanded, _, ok := s.TreeRowShape(s.Cursor); ok && expanded {
			return e.Path
		}
	}
	return s.CursorDir().String()
}

// otherPanelMatchNames returns, for panelID, the entry names directly in the other panel's
// otherMatchDir, the active panel's cursor entry name, and panelID's own otherMatchDir (rows
// outside it are not marked); nil names when the mark is off or both dirs are equal.
// ponytail: rebuilt per render (O(n) over the other listing); cache on State keyed by listing generation if huge dirs lag.
func otherPanelMatchNames(model Model, panelID int) (names map[string]struct{}, cursor, dir string) {
	if !model.OtherPanelMatchMarks {
		return nil, "", ""
	}
	self, other := &model.Primary, &model.Secondary
	if panelID == SecondaryPanel {
		self, other = other, self
	}
	dir, otherDir := otherMatchDir(self), otherMatchDir(other)
	if dir == otherDir || len(other.Entries) == 0 {
		return nil, "", ""
	}
	names = make(map[string]struct{}, len(other.Entries))
	add := func(e localfs.Entry) {
		if e.Name != ".." && other.EntryDir(e).String() == otherDir {
			names[e.Name] = struct{}{}
		}
	}
	if other.ListLayout == panel.ListLayoutTree {
		// Expanded children live only in the tree rows; a filter there hides names from matching.
		for i := range other.VisibleEntryCount() {
			if e, _, ok := other.VisibleEntry(i); ok {
				add(e)
			}
		}
	} else {
		for _, e := range other.Entries {
			add(e)
		}
	}
	active := &model.Primary
	if model.ActivePanel == SecondaryPanel {
		active = &model.Secondary
	}
	if e, ok := active.CurrentEntry(); ok && active.EntryDir(e).String() == otherMatchDir(active) {
		cursor = e.Name
	}
	return names, cursor, dir
}
