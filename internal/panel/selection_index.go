package panel

import (
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

func (s *State) rebuildListingByPath() {
	if len(s.Entries) == 0 {
		s.listingByPath = nil
		return
	}
	s.listingByPath = make(map[string]localfs.Entry, len(s.Entries))
	for _, e := range s.Entries {
		s.listingByPath[e.Path] = e
	}
}

// EntriesByPath returns a lookup map from absolute path to Entry for the panel's currently
// listed entries, for callers that need to resolve several paths against the listing (delete
// confirmation and selection-size disk-usage scan reconciliation).
func (s *State) EntriesByPath() map[string]localfs.Entry {
	byPath := make(map[string]localfs.Entry, len(s.Entries))
	for _, e := range s.Entries {
		byPath[e.Path] = e
	}
	return byPath
}

func (s *State) listingEntry(path string) (localfs.Entry, bool) {
	if e, ok := s.listingByPath[path]; ok {
		return e, true
	}
	e, ok := s.treeByPath[path]
	return e, ok
}

// OffListingMetaAt returns metadata for a selected path that is not in the current listing, as
// filled in by SetOffListingMeta (background Lstat; never touches the filesystem itself).
func (s *State) OffListingMetaAt(path string) (localfs.Entry, bool) {
	e, ok := s.offListingMeta[path]
	return e, ok
}

// SetOffListingMeta replaces the off-listing metadata cache (see OffListingMetaAt).
func (s *State) SetOffListingMeta(meta map[string]localfs.Entry) {
	s.offListingMeta = meta
}

func (s *State) markSelectedDir(path string) {
	if s.SelectedDirPaths == nil {
		s.SelectedDirPaths = make(map[string]bool)
	}
	s.SelectedDirPaths[path] = true
	s.selectionHasDirs = true
}

func (s *State) unmarkSelectedDir(path string) {
	if s.SelectedDirPaths == nil {
		return
	}
	delete(s.SelectedDirPaths, path)
	if len(s.SelectedDirPaths) == 0 {
		s.SelectedDirPaths = nil
		s.selectionHasDirs = false
	}
}

func (s *State) rebuildSelectedDirPaths() {
	s.SelectedDirPaths = nil
	s.selectionHasDirs = false
	if len(s.SelectedPaths) == 0 {
		return
	}
	for p, on := range s.SelectedPaths {
		if !on {
			continue
		}
		if e, ok := s.listingEntry(p); ok {
			if e.Type == localfs.EntryDirectory {
				s.markSelectedDir(p)
			}
			continue
		}
		if s.selectedPathIsDirectory(p) {
			s.markSelectedDir(p)
		}
	}
}

func (s *State) clearSelectionState() {
	s.SelectedPaths = nil
	s.SelectedDirPaths = nil
	s.selectionHasDirs = false
	s.selectionListedBytes = 0
	s.offListingMeta = nil
}

// clearSelectionStrictDescendantsIndexed removes selected paths strictly under parent.
func (s *State) clearSelectionStrictDescendantsIndexed(parent string) []string {
	if len(s.SelectedPaths) == 0 {
		return nil
	}
	parent = cleanPathString(parent)
	if parent == "" {
		return nil
	}
	prefix := filepath.ToSlash(parent) + "/"
	var removed []string
	for p := range s.SelectedPaths {
		clean := cleanPathString(p)
		if clean == "" {
			continue
		}
		if !pathHasSlashPrefix(filepath.ToSlash(clean), prefix) {
			continue
		}
		removed = append(removed, p)
		delete(s.SelectedPaths, p)
		if s.SelectedDirPaths != nil {
			delete(s.SelectedDirPaths, p)
		}
		s.adjustSelectionListedBytes(p, false)
	}
	if len(s.SelectedPaths) == 0 {
		s.clearSelectionState()
	} else if len(removed) > 0 && s.selectionHasDirs && len(s.SelectedDirPaths) == 0 {
		s.selectionHasDirs = false
	}
	return removed
}

func pathHasSlashPrefix(path, prefix string) bool {
	if len(path) < len(prefix) {
		return false
	}
	return path[:len(prefix)] == prefix
}

func (s *State) adjustSelectionListedBytes(path string, add bool) {
	e, ok := s.listingEntry(path)
	if !ok || e.Type == localfs.EntryDirectory {
		return
	}
	if add {
		s.selectionListedBytes += e.Size
	} else {
		s.selectionListedBytes -= e.Size
		if s.selectionListedBytes < 0 {
			s.selectionListedBytes = 0
		}
	}
}

func (s *State) recomputeSelectionListedBytes() {
	if len(s.SelectedPaths) == 0 || (s.listingByPath == nil && s.treeByPath == nil) {
		s.selectionListedBytes = 0
		return
	}
	var total int64
	for p, on := range s.SelectedPaths {
		if !on {
			continue
		}
		if e, ok := s.listingEntry(p); ok && e.Type != localfs.EntryDirectory {
			total += e.Size
		}
	}
	s.selectionListedBytes = total
}

// ListingEntryAt returns the current listing entry for path when present, including loaded
// tree-mode children.
func (s *State) ListingEntryAt(path string) (localfs.Entry, bool) {
	return s.listingEntry(path)
}

// SelectionListedBytes returns the cached sum of file sizes for selected in-listing files.
func (s State) SelectionListedBytes() int64 {
	return s.selectionListedBytes
}

// SelectionHasDirs reports whether any selected path is a directory.
func (s State) SelectionHasDirs() bool {
	return s.selectionHasDirs || len(s.SelectedDirPaths) > 0
}

// RemoveVanishedSelections deselects paths that no longer exist on disk (detected by the
// background selection-size Lstat pass) and returns how many were still selected and removed.
func (s *State) RemoveVanishedSelections(paths []string) int {
	removed := 0
	for _, p := range paths {
		p = cleanPathString(p)
		if !s.SelectedPaths[p] {
			continue
		}
		s.applySelectionRemove(p, s.SelectedDirPaths[p])
		s.removePathFromSelectionsStripOrder(p)
		removed++
	}
	if removed > 0 {
		s.normalizeSelectionsStripCursor()
	}
	return removed
}
