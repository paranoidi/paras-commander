package panel

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

// RemoveEntriesByPath drops matching rows from the in-memory listing immediately (no disk I/O).
// Used when enqueueing move/delete/flatten so selected names vanish before the async reload lands.
// Returns true when anything was removed.
func (s *State) RemoveEntriesByPath(paths []string, viewportRows int) bool {
	if s == nil || len(paths) == 0 {
		return false
	}
	drop := make(map[string]bool, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		drop[filepath.Clean(p)] = true
	}
	if len(drop) == 0 {
		return false
	}

	// Prefer retaining the highlighted entry by path (not index): pruning rows above the
	// cursor must not leave the highlight on a different name. If the focused row itself
	// was removed, fall back to the prior visible index (clamped).
	priorCursor := s.Cursor
	focusPath := ""
	if entry, ok := s.CurrentEntry(); ok {
		focusPath = entry.Path
	}

	kept := make([]localfs.Entry, 0, len(s.Entries))
	removedNames := make([]string, 0, len(drop))
	removed := false
	for _, e := range s.Entries {
		if drop[filepath.Clean(e.Path)] {
			removed = true
			removedNames = append(removedNames, e.Name)
			continue
		}
		kept = append(kept, e)
	}
	if !removed && s.ListLayout != ListLayoutTree {
		return false
	}

	s.Entries = kept
	treeRemoved := false
	if s.ListLayout == ListLayoutTree {
		treeRemoved = s.pruneTreeByPaths(drop)
	}
	if !removed && !treeRemoved {
		return false
	}
	if len(removedNames) > 0 {
		// A stale reload can land after this optimistic prune but before the job's real
		// filesystem op completes, still seeing the file on disk — without this, that
		// reappearance reads as newly created (see newlyAppearedNames / rename_marks.go).
		s.MarkPendingRemoval(s.Path, removedNames)
	}

	s.ListingEpoch++
	s.rebuildListingByPath()
	s.recomputeSelectionListedBytes()
	s.ApplySort()
	if s.ListLayout == ListLayoutTree {
		s.resyncTreeOrder()
	}
	s.rebuildFilter()

	restored := false
	if focusPath != "" && !drop[filepath.Clean(focusPath)] {
		s.selectVisibleEntryByPath(focusPath)
		if entry, ok := s.CurrentEntry(); ok && filepath.Clean(entry.Path) == filepath.Clean(focusPath) {
			restored = true
		}
	}
	if !restored {
		s.Cursor = priorCursor
		s.clampCursor()
	}
	s.EnsureCursorInViewport(s.viewportRowsOr(viewportRows))
	s.syncTreeCursorIDToCursor()
	return true
}

// RenameEntry renames a row in the in-memory listing (same parent directory). Returns true when
// the entry was found and updated.
func (s *State) RenameEntry(oldPath, newName string, viewportRows int) bool {
	if s == nil || oldPath == "" || newName == "" {
		return false
	}
	priorCursor := s.Cursor
	oldClean := filepath.Clean(oldPath)
	newPath := filepath.Join(filepath.Dir(oldClean), newName)
	found := false
	for i := range s.Entries {
		if filepath.Clean(s.Entries[i].Path) != oldClean {
			continue
		}
		s.Entries[i].Name = newName
		s.Entries[i].Path = newPath
		found = true
		break
	}
	if s.ListLayout == ListLayoutTree {
		if renameTreeNode(s.TreeRoots, oldClean, newName, newPath) {
			found = true
		}
	}
	if !found {
		return false
	}
	s.ListingEpoch++
	if s.SelectedPaths != nil && s.SelectedPaths[oldClean] {
		delete(s.SelectedPaths, oldClean)
		s.SelectedPaths[newPath] = true
		if s.SelectedDirPaths != nil && s.SelectedDirPaths[oldClean] {
			delete(s.SelectedDirPaths, oldClean)
			s.SelectedDirPaths[newPath] = true
		}
	}
	s.rebuildListingByPath()
	s.recomputeSelectionListedBytes()
	s.ApplySort()
	if s.ListLayout == ListLayoutTree {
		s.resyncTreeOrder()
	}
	s.rebuildFilter()
	// Keep the prior row index (MC-style); focus-after-rename selects by name via Refresh hook.
	s.Cursor = priorCursor
	s.clampCursor()
	s.EnsureCursorInViewport(s.viewportRowsOr(viewportRows))
	s.syncTreeCursorIDToCursor()
	return true
}

// InsertEntry adds an entry to the in-memory listing when its parent directory is this panel's
// Path. No-op when the row already exists or the parent does not match. Returns true when inserted.
func (s *State) InsertEntry(entry localfs.Entry, viewportRows int) bool {
	if s == nil || entry.Path == "" || s.Path.IsZero() {
		return false
	}
	loc, err := pathloc.Parse(entry.Path)
	if err != nil {
		return false
	}
	if !loc.Parent().Equal(s.Path) {
		return false
	}
	clean := filepath.Clean(entry.Path)
	for _, e := range s.Entries {
		if filepath.Clean(e.Path) == clean {
			return false
		}
	}
	if entry.Name == "" {
		entry.Name = loc.Base()
	}
	entry.Path = clean
	s.Entries = append(s.Entries, entry)
	s.ListingEpoch++
	s.rebuildListingByPath()
	s.recomputeSelectionListedBytes()
	s.ApplySort()
	if s.ListLayout == ListLayoutTree {
		s.resyncTreeOrder()
	}
	s.rebuildFilter()
	s.clampCursor()
	s.EnsureCursorInViewport(s.viewportRowsOr(viewportRows))
	return true
}

func (s *State) viewportRowsOr(viewportRows int) int {
	if viewportRows > 0 {
		return viewportRows
	}
	if s.FileListViewportRows != nil {
		return s.FileListViewportRows()
	}
	return 0
}

func (s *State) pruneTreeByPaths(drop map[string]bool) bool {
	if s.ListLayout != ListLayoutTree || len(s.TreeRoots) == 0 {
		return false
	}
	beforeRoots := len(s.TreeRoots)
	beforeRows := len(s.treeRows)
	s.TreeRoots = pruneTreeNodes(s.TreeRoots, drop)
	for id := range s.TreeExpanded {
		if drop[id] {
			delete(s.TreeExpanded, id)
		}
	}
	s.rebuildTreeRows()
	return len(s.TreeRoots) != beforeRoots || len(s.treeRows) != beforeRows
}

func pruneTreeNodes(nodes []treeflat.Node[TreeEntry], drop map[string]bool) []treeflat.Node[TreeEntry] {
	if len(nodes) == 0 {
		return nodes
	}
	out := make([]treeflat.Node[TreeEntry], 0, len(nodes))
	for _, n := range nodes {
		id := filepath.Clean(n.ID)
		if drop[id] || drop[filepath.Clean(n.Value.Entry.Path)] {
			continue
		}
		if n.Children != nil {
			n.Children = pruneTreeNodes(n.Children, drop)
		}
		out = append(out, n)
	}
	return out
}

func renameTreeNode(nodes []treeflat.Node[TreeEntry], oldPath, newName, newPath string) bool {
	for i := range nodes {
		if filepath.Clean(nodes[i].ID) == oldPath || filepath.Clean(nodes[i].Value.Entry.Path) == oldPath {
			nodes[i].ID = newPath
			nodes[i].Value.Entry.Name = newName
			nodes[i].Value.Entry.Path = newPath
			return true
		}
		if renameTreeNode(nodes[i].Children, oldPath, newName, newPath) {
			return true
		}
	}
	return false
}
