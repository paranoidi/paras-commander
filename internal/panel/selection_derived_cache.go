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
	s.selDerivedCache.built = false
}

func (s *State) invalidateSelectionDerivedFull() {
	s.invalidateSelectionDerived()
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

func (s *State) patchSelectionDerivedAfterAdd(path string, isDir bool) {
	cur := cleanPathString(s.Path.String())
	if !s.selDerivedCache.built || s.selDerivedCache.path != cur {
		s.rebuildSelectionDerived()
		return
	}
	path = cleanPathString(path)
	if path == "" {
		s.invalidateSelectionDerived()
		return
	}
	s.addSubtreeAncestorsForPath(path)
	if len(s.selDerivedCache.stripPaths) > 0 {
		s.appendStripPathIfNeeded(path)
	} else if cleanPathString(filepath.Dir(path)) != cur {
		// Strip just became visible: it lists every selection, so rebuild.
		s.rebuildSelectionDerived()
		return
	}
	s.patchPrunedRootsAfterAdd(path, isDir)
	s.selDerivedCache.gen = s.selectionDerivedGen
}

func (s *State) patchSelectionDerivedAfterRemove(path string, wasDir bool) {
	cur := cleanPathString(s.Path.String())
	if !s.selDerivedCache.built || s.selDerivedCache.path != cur {
		s.rebuildSelectionDerived()
		return
	}
	path = cleanPathString(path)
	if path == "" {
		s.invalidateSelectionDerived()
		return
	}
	s.removeSubtreeAncestorsForPath(path)
	s.removeStripPath(path)
	if !selectionsOutsideDir(s.SelectedPaths, cur) {
		// Last out-of-directory selection gone: strip hides.
		s.selDerivedCache.stripPaths = nil
	}
	s.patchPrunedRootsAfterRemove(path, wasDir)
	s.selDerivedCache.gen = s.selectionDerivedGen
}

func (s *State) addSubtreeAncestorsForPath(path string) {
	if s.selDerivedCache.subtreeAncestors == nil {
		s.selDerivedCache.subtreeAncestors = make(map[string]bool)
	}
	loc, err := pathloc.Parse(path)
	if err != nil {
		return
	}
	for {
		parent := loc.Parent()
		if parent.Equal(loc) || parent.IsZero() {
			break
		}
		s.selDerivedCache.subtreeAncestors[cleanPathString(parent.String())] = true
		loc = parent
	}
}

func (s *State) removeSubtreeAncestorsForPath(path string) {
	if s.SelectedPaths == nil || s.selDerivedCache.subtreeAncestors == nil {
		return
	}
	loc, err := pathloc.Parse(path)
	if err != nil {
		return
	}
	for {
		parent := loc.Parent()
		if parent.Equal(loc) || parent.IsZero() {
			break
		}
		ps := cleanPathString(parent.String())
		if s.stillHasSelectionUnderAncestor(ps, path) {
			loc = parent
			continue
		}
		delete(s.selDerivedCache.subtreeAncestors, ps)
		loc = parent
	}
	if len(s.selDerivedCache.subtreeAncestors) == 0 {
		s.selDerivedCache.subtreeAncestors = nil
	}
}

func (s *State) stillHasSelectionUnderAncestor(ancestor, exceptPath string) bool {
	for p, on := range s.SelectedPaths {
		if !on || p == exceptPath {
			continue
		}
		if isStrictPathDescendant(ancestor, p) {
			return true
		}
	}
	return false
}

func (s *State) appendStripPathIfNeeded(path string) {
	for _, p := range s.selDerivedCache.stripPaths {
		if p == path {
			return
		}
	}
	s.selDerivedCache.stripPaths = append(s.selDerivedCache.stripPaths, path)
	sort.Strings(s.selDerivedCache.stripPaths)
}

func (s *State) removeStripPath(path string) {
	if len(s.selDerivedCache.stripPaths) == 0 {
		return
	}
	out := s.selDerivedCache.stripPaths[:0]
	for _, p := range s.selDerivedCache.stripPaths {
		if p != path {
			out = append(out, p)
		}
	}
	s.selDerivedCache.stripPaths = out
}

func (s *State) patchPrunedRootsAfterAdd(path string, isDir bool) {
	if !s.selectionHasDirs || isDir {
		s.selDerivedCache.prunedRoots = insertPrunedRoot(s.selDerivedCache.prunedRoots, path, isDir)
		return
	}
	s.selDerivedCache.prunedRoots = insertSortedUnique(s.selDerivedCache.prunedRoots, path)
}

func (s *State) patchPrunedRootsAfterRemove(path string, wasDir bool) {
	if !s.selectionHasDirs && !wasDir {
		s.selDerivedCache.prunedRoots = removeSortedPath(s.selDerivedCache.prunedRoots, path)
		return
	}
	if len(s.SelectedPaths) == 0 {
		s.selDerivedCache.prunedRoots = nil
		return
	}
	s.selDerivedCache.prunedRoots = s.buildPrunedSelectionRoots()
}

func insertSortedUnique(sorted []string, path string) []string {
	path = cleanPathString(path)
	if path == "" {
		return sorted
	}
	i := sort.SearchStrings(sorted, path)
	if i < len(sorted) && sorted[i] == path {
		return sorted
	}
	out := make([]string, 0, len(sorted)+1)
	out = append(out, sorted[:i]...)
	out = append(out, path)
	out = append(out, sorted[i:]...)
	return out
}

func removeSortedPath(sorted []string, path string) []string {
	path = cleanPathString(path)
	if path == "" || len(sorted) == 0 {
		return sorted
	}
	i := sort.SearchStrings(sorted, path)
	if i >= len(sorted) || sorted[i] != path {
		return sorted
	}
	out := make([]string, 0, len(sorted)-1)
	out = append(out, sorted[:i]...)
	out = append(out, sorted[i+1:]...)
	return out
}

func insertPrunedRoot(roots []string, path string, isDir bool) []string {
	path = cleanPathString(path)
	if path == "" {
		return roots
	}
	if !isDir {
		return insertSortedUnique(roots, path)
	}
	combined := PruneNestedPaths(append(append([]string(nil), roots...), path))
	return combined
}

func (s *State) applySelectionAdd(path string, isDir bool) {
	path = cleanPathString(path)
	if path == "" {
		return
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	s.SelectedPaths[path] = true
	if isDir {
		s.markSelectedDir(path)
	}
	s.adjustSelectionListedBytes(path, true)
	s.selectionDerivedGen++
	s.patchSelectionDerivedAfterAdd(path, isDir)
}

func (s *State) applySelectionRemove(path string, wasDir bool) {
	path = cleanPathString(path)
	if path == "" || s.SelectedPaths == nil {
		return
	}
	delete(s.SelectedPaths, path)
	s.adjustSelectionListedBytes(path, false)
	if wasDir {
		s.unmarkSelectedDir(path)
	}
	if len(s.SelectedPaths) == 0 {
		s.clearSelectionState()
		s.invalidateSelectionDerived()
		return
	}
	s.selectionDerivedGen++
	s.patchSelectionDerivedAfterRemove(path, wasDir)
}

// buildSelectionsStripPaths lists ALL selected paths (order-first, extras sorted) when at
// least one selection lives outside cur; an all-in-current-directory selection hides the strip.
func (s *State) buildSelectionsStripPaths(cur string) []string {
	if !selectionsOutsideDir(s.SelectedPaths, cur) {
		return nil
	}
	seen := make(map[string]bool, len(s.SelectedPaths))
	out := make([]string, 0, len(s.SelectedPaths))
	for _, p := range s.SelectionsStripOrder {
		if !s.SelectedPaths[p] || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	extra := make([]string, 0)
	for p := range s.SelectedPaths {
		if seen[p] {
			continue
		}
		extra = append(extra, p)
	}
	sort.Strings(extra)
	return append(out, extra...)
}

func selectionsOutsideDir(selected map[string]bool, dir string) bool {
	for p, on := range selected {
		if on && cleanPathString(filepath.Dir(p)) != dir {
			return true
		}
	}
	return false
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
	s.rebuildSelectedDirPaths()
	s.recomputeSelectionListedBytes()
	s.invalidateSelectionDerivedFull()
	return removed
}
