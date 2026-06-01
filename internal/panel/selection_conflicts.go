package panel

import (
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// ClearSelectionConflicts removes paths in selected that conflict with adding path.
// When adding path P: strict descendants of P are removed; selected directory ancestors of P are removed.
// addedIsDir is the type of P (reserved for future rules). existingIsDir reports whether a selected path is a directory.
// Returns true if any path was removed.
func ClearSelectionConflicts(selected map[string]bool, path string, addedIsDir bool, existingIsDir func(string) bool) bool {
	_ = addedIsDir
	if len(selected) == 0 {
		return false
	}
	added := cleanPathString(path)
	if added == "" {
		return false
	}
	if existingIsDir == nil {
		existingIsDir = func(string) bool { return false }
	}
	var removed bool
	for p := range selected {
		clean := cleanPathString(p)
		if clean == "" {
			continue
		}
		if IsStrictPathDescendant(added, clean) {
			delete(selected, p)
			removed = true
			continue
		}
		if existingIsDir(clean) && IsStrictPathDescendant(clean, added) {
			delete(selected, p)
			removed = true
		}
	}
	return removed
}

func (s *State) selectedPathIsDirectory(path string) bool {
	path = cleanPathString(path)
	if path == "" {
		return false
	}
	for _, e := range s.Entries {
		if e.Path == path {
			return e.Type == localfs.EntryDirectory
		}
	}
	e, err := localfs.EntryFromPath(path)
	if err != nil {
		return false
	}
	return e.Type == localfs.EntryDirectory
}

func (s *State) resolveSelectionConflicts(path string, addedIsDir bool) bool {
	if len(s.SelectedPaths) == 0 {
		return false
	}
	before := make([]string, 0, len(s.SelectedPaths))
	for p := range s.SelectedPaths {
		before = append(before, p)
	}
	if !ClearSelectionConflicts(s.SelectedPaths, path, addedIsDir, s.selectedPathIsDirectory) {
		return false
	}
	for _, p := range before {
		if s.SelectedPaths[p] {
			continue
		}
		s.removePathFromSelectionsStripOrder(p)
	}
	if len(s.SelectedPaths) == 0 {
		s.SelectedPaths = nil
	}
	return true
}
