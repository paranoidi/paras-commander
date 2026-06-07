package panel

import (
	"path/filepath"
	"sort"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

type selectionDerivedCache struct {
	built            bool
	path             string
	gen              uint64
	stripPaths       []string
	prunedRoots      []string
	subtreeAncestors map[string]bool
}

// SelectionDerivedGen returns the selection-derived cache generation (bumped on selection mutations).
func (s State) SelectionDerivedGen() uint64 {
	return s.selectionDerivedGen
}

func (s *State) invalidateSelectionDerived() {
	s.selectionDerivedGen++
}

func (s *State) selDerivedValid() bool {
	return s.selDerivedCache.built &&
		s.selDerivedCache.gen == s.selectionDerivedGen &&
		s.selDerivedCache.path == cleanPathString(s.Path.String())
}

func (s *State) ensureSelectionDerived() {
	if s.selDerivedValid() {
		return
	}
	s.rebuildSelectionDerived()
}

func (s *State) rebuildSelectionDerived() {
	cur := cleanPathString(s.Path.String())
	cache := selectionDerivedCache{
		built: true,
		path:  cur,
		gen:   s.selectionDerivedGen,
	}

	cache.stripPaths = s.buildSelectionsStripPaths(cur)
	cache.prunedRoots = s.buildPrunedSelectionRoots()
	cache.subtreeAncestors = buildSubtreeSelAncestors(s.SelectedPaths)

	s.selDerivedCache = cache
}

func (s *State) buildSelectionsStripPaths(cur string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(s.SelectionsStripOrder))
	for _, p := range s.SelectionsStripOrder {
		if s.SelectedPaths == nil || !s.SelectedPaths[p] {
			continue
		}
		if cleanPathString(filepath.Dir(p)) == cur {
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	if s.SelectedPaths != nil {
		extra := make([]string, 0)
		for p := range s.SelectedPaths {
			if cleanPathString(filepath.Dir(p)) == cur {
				continue
			}
			if seen[p] {
				continue
			}
			extra = append(extra, p)
		}
		if len(extra) > 0 {
			sort.Strings(extra)
			out = append(out, extra...)
		}
	}
	return out
}

func (s *State) buildPrunedSelectionRoots() []string {
	if len(s.SelectedPaths) == 0 {
		return nil
	}
	paths := make([]string, 0, len(s.SelectedPaths))
	for p, on := range s.SelectedPaths {
		if on {
			paths = append(paths, p)
		}
	}
	filesOnly := !s.selectionHasDirs
	return PruneNestedPathsForSelection(paths, filesOnly, s.selectedPathIsDirectory)
}

func (s *State) refreshSelectionHasDirs(isDir func(string) bool) {
	if isDir == nil {
		isDir = s.selectedPathIsDirectory
	}
	s.selectionHasDirs = false
	if len(s.SelectedPaths) == 0 {
		return
	}
	for p, on := range s.SelectedPaths {
		if on && isDir(p) {
			s.selectionHasDirs = true
			return
		}
	}
}

func buildSubtreeSelAncestors(selected map[string]bool) map[string]bool {
	if len(selected) == 0 {
		return nil
	}
	out := make(map[string]bool)
	for p, on := range selected {
		if !on {
			continue
		}
		loc, err := pathloc.Parse(p)
		if err != nil {
			continue
		}
		for {
			parent := loc.Parent()
			if parent.Equal(loc) || parent.IsZero() {
				break
			}
			out[cleanPathString(parent.String())] = true
			loc = parent
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PrunedSelectionRoots returns conflict-pruned selected paths for size labels and disk scans.
func (s *State) PrunedSelectionRoots() []string {
	s.ensureSelectionDerived()
	return s.selDerivedCache.prunedRoots
}

// PruneNestedPathsForSelection prunes nested paths. When filesOnly is true, paths are treated as
// files only (no directory nesting) and pruning is O(n log n).
func PruneNestedPathsForSelection(paths []string, filesOnly bool, isDir func(string) bool) []string {
	if filesOnly {
		return sortedUniqueSelectionPaths(paths)
	}
	if !filesOnly && isDir != nil {
		hasDir := false
		for _, p := range paths {
			if isDir(p) {
				hasDir = true
				break
			}
		}
		if !hasDir {
			return sortedUniqueSelectionPaths(paths)
		}
	}
	return PruneNestedPaths(paths)
}

func sortedUniqueSelectionPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	sorted := make([]string, len(paths))
	for i, p := range paths {
		sorted[i] = cleanPathString(p)
	}
	sort.Strings(sorted)
	out := make([]string, 0, len(sorted))
	var prev string
	for _, p := range sorted {
		if p == "" {
			continue
		}
		if p == prev {
			continue
		}
		out = append(out, p)
		prev = p
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// BulkAddSelections marks paths in one pass using conflict rules. Returns true if conflicts were removed.
// isDir reports whether a path is a directory; nil uses the panel listing and filesystem stat.
func (s *State) BulkAddSelections(paths []string, isDir func(string) bool) bool {
	if len(paths) == 0 {
		return false
	}
	if isDir == nil {
		isDir = s.selectedPathIsDirectory
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool, len(paths))
	}

	removed := BulkApplySelectionAdds(s.SelectedPaths, paths, isDir)
	s.refreshSelectionHasDirs(isDir)
	s.invalidateSelectionDerived()
	return removed
}
