package panel

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// clearSelectionDirAncestors removes selected directory ancestors of path.
func clearSelectionDirAncestors(selected map[string]bool, path string, isDir func(string) bool) bool {
	if len(selected) == 0 {
		return false
	}
	path = cleanPathString(path)
	if path == "" {
		return false
	}
	if isDir == nil {
		isDir = func(string) bool { return false }
	}
	loc, err := pathloc.Parse(path)
	if err != nil {
		return false
	}
	var removed bool
	for {
		parent := loc.Parent()
		if parent.Equal(loc) || parent.IsZero() {
			break
		}
		ps := cleanPathString(parent.String())
		if ps != "" && selected[ps] && isDir(ps) {
			delete(selected, ps)
			removed = true
		}
		loc = parent
	}
	return removed
}

// clearSelectionStrictDescendants removes selected paths strictly under parent.
func clearSelectionStrictDescendants(selected map[string]bool, parent string) bool {
	if len(selected) == 0 {
		return false
	}
	parent = cleanPathString(parent)
	if parent == "" {
		return false
	}
	prefix := filepath.ToSlash(parent) + "/"
	var removed bool
	for p := range selected {
		clean := cleanPathString(p)
		if clean == "" {
			continue
		}
		if strings.HasPrefix(filepath.ToSlash(clean), prefix) {
			delete(selected, p)
			removed = true
		}
	}
	return removed
}

// ClearSelectionConflicts removes paths in selected that conflict with adding path.
// When adding path P: strict descendants of P are removed; selected directory ancestors of P are removed.
// addedIsDir is the type of P (reserved for future rules). existingIsDir reports whether a selected path is a directory.
// Returns true if any path was removed.
func ClearSelectionConflicts(selected map[string]bool, path string, addedIsDir bool, existingIsDir func(string) bool) bool {
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
	if addedIsDir {
		if clearSelectionStrictDescendants(selected, added) {
			removed = true
		}
	}
	if clearSelectionDirAncestors(selected, added, existingIsDir) {
		removed = true
	}
	return removed
}

// selectionMapHasDirs reports whether selected contains any marked directory path.
func selectionMapHasDirs(selected map[string]bool, isDir func(string) bool) bool {
	if len(selected) == 0 {
		return false
	}
	if isDir == nil {
		isDir = func(string) bool { return false }
	}
	for p, on := range selected {
		if on && isDir(p) {
			return true
		}
	}
	return false
}

// BulkApplySelectionAdds marks paths using the same conflict rules as ApplySelectionAdds.
// When all new paths are files and selected has no directories, uses O(n) direct inserts.
// Returns true if any conflicting path was removed from selected.
func BulkApplySelectionAdds(selected map[string]bool, paths []string, isDir func(string) bool) bool {
	if len(paths) == 0 {
		return false
	}
	if isDir == nil {
		isDir = func(string) bool { return false }
	}
	allNewFiles := true
	for _, path := range paths {
		if isDir(path) {
			allNewFiles = false
			break
		}
	}
	if allNewFiles && !selectionMapHasDirs(selected, isDir) {
		for _, path := range paths {
			path = cleanPathString(path)
			if path == "" {
				continue
			}
			selected[path] = true
		}
		return false
	}
	return applySelectionAddsBulk(selected, paths, isDir)
}

// applySelectionAddsBulk marks a batch of paths using the same conflict rules as repeated
// single-path adds (ClearSelectionConflicts), but in O((n+m)*log(m) + (n+m)*depth) instead of
// the O(n^2) cost of resolving conflicts one path at a time against the whole selection. Correct
// for any input order and any pre-existing selection:
//
//   - Phase 1 clears existing selected paths that are strict descendants of a directory being
//     added, in one pass over the pre-existing selection (independent of batch size).
//   - Phase 2 sorts the batch by depth ascending so every directory is processed before its own
//     descendants regardless of the caller's input order, then adds it same as
//     BulkApplySelectionAddsWalkOrder (ancestor-clearing on each add already handles the reverse
//     direction: a newly added path evicts an already-selected covering ancestor directory).
func applySelectionAddsBulk(selected map[string]bool, paths []string, isDir func(string) bool) bool {
	if len(paths) == 0 {
		return false
	}
	if isDir == nil {
		isDir = func(string) bool { return false }
	}
	cleaned := make([]string, 0, len(paths))
	newDirSet := make(map[string]bool)
	for _, path := range paths {
		path = cleanPathString(path)
		if path == "" {
			continue
		}
		cleaned = append(cleaned, path)
		if isDir(path) {
			newDirSet[path] = true
		}
	}
	if len(cleaned) == 0 {
		return false
	}
	var removed bool
	if len(newDirSet) > 0 && len(selected) > 0 {
		for p := range selected {
			if pathHasAncestorIn(p, newDirSet) {
				delete(selected, p)
				removed = true
			}
		}
	}
	sort.SliceStable(cleaned, func(i, j int) bool {
		return pathDepth(cleaned[i]) < pathDepth(cleaned[j])
	})
	for _, path := range cleaned {
		if clearSelectionDirAncestors(selected, path, isDir) {
			removed = true
		}
		selected[path] = true
	}
	return removed
}

// pathHasAncestorIn reports whether any strict ancestor of path is present in set.
func pathHasAncestorIn(path string, set map[string]bool) bool {
	if len(set) == 0 {
		return false
	}
	loc, err := pathloc.Parse(path)
	if err != nil {
		return false
	}
	for {
		parent := loc.Parent()
		if parent.Equal(loc) || parent.IsZero() {
			return false
		}
		if set[cleanPathString(parent.String())] {
			return true
		}
		loc = parent
	}
}

// BulkApplySelectionAddsWalkOrder marks paths assuming parents appear before descendants
// (find WalkDir index order). Only valid when selected starts empty or caller accepts walk-order semantics.
// Returns true if any conflicting path was removed from selected.
func BulkApplySelectionAddsWalkOrder(selected map[string]bool, paths []string, isDir func(string) bool) bool {
	if len(paths) == 0 {
		return false
	}
	if isDir == nil {
		isDir = func(string) bool { return false }
	}
	var removed bool
	for _, path := range paths {
		path = cleanPathString(path)
		if path == "" {
			continue
		}
		if clearSelectionDirAncestors(selected, path, isDir) {
			removed = true
		}
		selected[path] = true
	}
	return removed
}

// ApplySelectionAdds marks paths in order using the same rules as repeated single-path adds.
// Returns true if any conflicting path was removed from selected.
func ApplySelectionAdds(selected map[string]bool, paths []string, isDir func(string) bool) bool {
	if len(paths) == 0 {
		return false
	}
	if isDir == nil {
		isDir = func(string) bool { return false }
	}
	var removed bool
	for _, path := range paths {
		path = cleanPathString(path)
		if path == "" {
			continue
		}
		if ClearSelectionConflicts(selected, path, isDir(path), isDir) {
			removed = true
		}
		selected[path] = true
	}
	return removed
}

// PruneSelectionConflicts reduces selected to a conflict-free set by replaying adds for
// every currently marked path in depth-descending order (deepest paths first).
// Returns true if any path was removed.
func PruneSelectionConflicts(selected map[string]bool, isDir func(string) bool) bool {
	if len(selected) == 0 {
		return false
	}
	paths := make([]string, 0, len(selected))
	for path, on := range selected {
		if on {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return false
	}
	pruned := make(map[string]bool, len(paths))
	ApplySelectionAdds(pruned, pathsByDepthDescending(paths), isDir)
	removed := len(pruned) != len(paths)
	if !removed {
		for path := range selected {
			if !pruned[path] {
				removed = true
				break
			}
		}
	}
	for path := range selected {
		delete(selected, path)
	}
	for path := range pruned {
		selected[path] = true
	}
	return removed
}

func pathsByDepthDescending(paths []string) []string {
	if len(paths) <= 1 {
		return paths
	}
	out := append([]string(nil), paths...)
	for i := range out {
		for j := i + 1; j < len(out); j++ {
			if pathDepth(out[j]) > pathDepth(out[i]) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func pathDepth(path string) int {
	path = cleanPathString(path)
	if path == "" {
		return 0
	}
	slash := strings.Count(filepath.ToSlash(path), "/")
	if slash == 0 {
		return 1
	}
	return slash + 1
}

func (s *State) selectedPathIsDirectory(path string) bool {
	path = cleanPathString(path)
	if path == "" {
		return false
	}
	if e, ok := s.listingEntry(path); ok {
		return e.Type == localfs.EntryDirectory
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
	if !addedIsDir && !s.selectionHasDirs {
		return false
	}
	added := cleanPathString(path)
	if added == "" {
		return false
	}
	var removed []string
	if addedIsDir {
		removed = append(removed, s.clearSelectionStrictDescendantsIndexed(added)...)
	}
	if s.selectionHasDirs {
		dirBefore := make([]string, 0, len(s.SelectedDirPaths))
		for p := range s.SelectedDirPaths {
			dirBefore = append(dirBefore, p)
		}
		isSelDir := func(p string) bool {
			return s.SelectedDirPaths != nil && s.SelectedDirPaths[p]
		}
		if clearSelectionDirAncestors(s.SelectedPaths, added, isSelDir) {
			for _, p := range dirBefore {
				if s.SelectedPaths == nil || !s.SelectedPaths[p] {
					removed = append(removed, p)
					s.unmarkSelectedDir(p)
					s.adjustSelectionListedBytes(p, false)
				}
			}
		}
	}
	if len(removed) == 0 {
		return false
	}
	for _, p := range removed {
		s.removePathFromSelectionsStripOrder(p)
	}
	if len(s.SelectedPaths) == 0 {
		s.clearSelectionState()
	}
	// Removals bypassed applySelectionRemove; drop the derived cache so the
	// follow-up add rebuilds strip paths without the conflicting entries.
	s.invalidateSelectionDerived()
	return true
}
