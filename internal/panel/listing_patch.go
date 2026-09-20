package panel

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

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
	return s.InsertEntries([]localfs.Entry{entry}, viewportRows)
}

// InsertEntries adds multiple entries to the in-memory listing in one batch (used by mkdir and
// duplicate via InsertEntry): entries whose parent directory isn't this panel's Path, or that
// already exist, are skipped, and the expensive index rebuild/sort/filter passes run once for
// the whole batch rather than once per entry — inserting one at a time (each doing its own
// O(entries) duplicate scan and full ApplySort) is O(n²) and stalls the UI on a large batch.
// Returns true when anything was inserted.
func (s *State) InsertEntries(entries []localfs.Entry, viewportRows int) bool {
	if s == nil || s.Path.IsZero() || len(entries) == 0 {
		return false
	}
	existing := make(map[string]bool, len(s.Entries)+len(entries))
	for _, e := range s.Entries {
		existing[filepath.Clean(e.Path)] = true
	}
	inserted := false
	for _, entry := range entries {
		if entry.Path == "" {
			continue
		}
		loc, err := pathloc.Parse(entry.Path)
		if err != nil || !loc.Parent().Equal(s.Path) {
			continue
		}
		clean := filepath.Clean(entry.Path)
		if existing[clean] {
			continue
		}
		existing[clean] = true
		if entry.Name == "" {
			entry.Name = loc.Base()
		}
		entry.Path = clean
		s.Entries = append(s.Entries, entry)
		inserted = true
	}
	if !inserted {
		return false
	}
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
