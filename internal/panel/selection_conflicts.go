package panel

import (
	"path/filepath"
	"strings"

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
