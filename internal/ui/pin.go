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
	names = other.NamesDirectlyIn(otherDir)
	active := &model.Primary
	if model.ActivePanel == SecondaryPanel {
		active = &model.Secondary
	}
	if e, ok := active.CurrentEntry(); ok && active.EntryDir(e).String() == otherMatchDir(active) {
		cursor = e.Name
	}
	return names, cursor, dir
}

// otherPanelMarksFollowCursor reports whether a cursor-only move in panelID can change what the
// other panel draws: the match marks are on and some row in the other panel's viewport (rows list
// rows starting at ScrollOffset, as drawPanel paints them) would carry a mark (same row test as
// drawPanelRow). A move in a tree-layout panel also shifts its match dir (otherMatchDir), which
// can add or clear marks, so that always counts.
func otherPanelMarksFollowCursor(model Model, panelID, rows int) bool {
	if !model.OtherPanelMatchMarks {
		return false
	}
	otherID := PrimaryPanel
	moved, other := &model.Secondary, &model.Primary
	if panelID == PrimaryPanel {
		otherID = SecondaryPanel
		moved, other = &model.Primary, &model.Secondary
	}
	if moved.ListLayout == panel.ListLayoutTree {
		return true
	}
	names, _, dir := otherPanelMatchNames(model, otherID)
	if len(names) == 0 {
		return false
	}
	end := min(other.ScrollOffset+rows, other.VisibleEntryCount())
	for i := max(other.ScrollOffset, 0); i < end; i++ {
		e, _, ok := other.VisibleEntry(i)
		if !ok || e.Name == ".." || other.EntryDir(e).String() != dir {
			continue
		}
		if _, hit := names[e.Name]; hit {
			return true
		}
	}
	return false
}
