package panel

import (
	"errors"
	"slices"
	"sort"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/selstrip"
	"github.com/paranoidi/paras-commander/internal/treeflat"
)

type selectionDerivedCache struct {
	built            bool
	path             string
	gen              uint64
	stripPaths       []string
	stripTree        *treeflat.PathTree // labels of stripPaths, kept in step with them; nil: built on the next tree-mode use
	stripRows        []selstrip.Row     // stripPaths as strip rows (flat or tree per Strip.Tree)
	stripRowsBuilt   bool
	stripRowsTree    bool
	stripRowsRev     int // Strip.CollapsedRev the rows were built with
	prunedRoots      []string
	subtreeAncestors map[string]int32 // dir -> selected paths strictly below it (no zero entries)
}

// SelectionDerivedGen returns the selection-derived cache generation (bumped on selection mutations).
func (s State) SelectionDerivedGen() uint64 {
	return s.selectionDerivedGen
}

func (s *State) invalidateSelectionDerived() {
	s.selectionDerivedGen++
	s.selDerivedCache.built = false
}

// SetStripForced shows (or stops forcing) the selected strip even when every
// selection lives in the current directory.
func (s *State) SetStripForced(on bool) {
	if s.StripForced == on {
		return
	}
	s.StripForced = on
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

	cache.stripPaths = s.buildSelectedStripPaths(cur)
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
		s.editStripPath(path, true)
	} else if parent, ok := selectionParentDir(path); s.StripForced || !ok || parent != cur {
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
	if !s.StripForced && !selectionsOutsideDir(s.SelectedPaths, cur) {
		// Last out-of-directory selection gone: strip hides.
		c := &s.selDerivedCache
		c.stripPaths, c.stripRowsBuilt, c.stripTree = nil, false, nil
	} else if len(s.selDerivedCache.stripPaths) > 0 {
		s.editStripPath(path, false)
	}
	s.patchPrunedRootsAfterRemove(path, wasDir)
	s.selDerivedCache.gen = s.selectionDerivedGen
}

// addSubtreeAncestorsForPath counts path under each of its ancestor directories.
func (s *State) addSubtreeAncestorsForPath(path string) {
	if s.selDerivedCache.subtreeAncestors == nil {
		s.selDerivedCache.subtreeAncestors = make(map[string]int32)
	}
	countSubtreeAncestors(s.selDerivedCache.subtreeAncestors, path, 1)
}

// removeSubtreeAncestorsForPath uncounts path under each of its ancestor directories, in
// O(depth): a directory whose count drops to zero has no selection below it any more.
func (s *State) removeSubtreeAncestorsForPath(path string) {
	if s.selDerivedCache.subtreeAncestors == nil {
		return
	}
	countSubtreeAncestors(s.selDerivedCache.subtreeAncestors, path, -1)
}

// countSubtreeAncestors adds delta to the count of every strict ancestor directory of path,
// deleting the counts that reach zero.
func countSubtreeAncestors(counts map[string]int32, path string, delta int32) {
	walkAncestors(path, func(dir string) bool {
		if n := counts[dir] + delta; n > 0 {
			counts[dir] = n
		} else {
			delete(counts, dir)
		}
		return false
	})
}

// editStripPath adds (on) or drops path in the strip's sorted list and its tree, when built.
func (s *State) editStripPath(path string, on bool) {
	c := &s.selDerivedCache
	c.stripPaths = selstrip.ApplyEdits(c.stripPaths, []selstrip.Edit{{Path: path, On: on}}, c.stripTree, s.Path.String())
	c.stripRowsBuilt = false
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
	if s.SelectedPaths[path] {
		return // already counted in the listed bytes and subtree counts
	}
	s.SelectedPaths[path] = true
	if isDir {
		s.markSelectedDir(path)
	}
	s.adjustSelectionListedBytes(path, true)
	s.selectionDerivedGen++
	s.patchSelectionDerivedAfterAdd(path, isDir)
}

// applySelectionRemove unselects one path and patches the derived state. A path that is not
// selected changes nothing (its counts and bytes were never added); whether it was a directory
// comes from SelectedDirPaths, never a stat.
func (s *State) applySelectionRemove(path string) {
	path = cleanPathString(path)
	if path == "" || !s.SelectedPaths[path] {
		return
	}
	wasDir := s.SelectedDirPaths[path]
	s.dropSelectedPath(path)
	if len(s.SelectedPaths) == 0 {
		s.clearSelectionState()
		s.invalidateSelectionDerived()
		return
	}
	s.selectionDerivedGen++
	s.patchSelectionDerivedAfterRemove(path, wasDir)
}

// buildSelectedStripPaths lists ALL selected paths, sorted, when at least one selection lives
// outside cur or StripForced is set; an all-in-current-directory selection yields nil (strip
// hidden). Sorted order is what editStripPath keeps.
func (s *State) buildSelectedStripPaths(cur string) []string {
	if !s.StripForced && !selectionsOutsideDir(s.SelectedPaths, cur) {
		return nil
	}
	out := make([]string, 0, len(s.SelectedPaths))
	for p, on := range s.SelectedPaths {
		if on {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return out
}

func selectionsOutsideDir(selected map[string]bool, dir string) bool {
	dir = cleanPathString(dir)
	for p, on := range selected {
		if !on {
			continue
		}
		parent, ok := selectionParentDir(p)
		if !ok {
			if cleanPathString(p) != dir {
				return true
			}
			continue
		}
		if parent != dir {
			return true
		}
	}
	return false
}

// selectionParentDir is the directory above path (pathloc.Path.Up: the list for a row of a flat tag
// view) using pathloc so sftp:// URIs resolve correctly. False when the path cannot be parsed or
// is already the root.
func selectionParentDir(path string) (string, bool) {
	loc, err := pathloc.Parse(path)
	if err != nil {
		return "", false
	}
	parent := loc.Up()
	if parent.Equal(loc) || parent.IsZero() {
		return "", false
	}
	return cleanPathString(parent.String()), true
}

// knownSelectionIsDir reports whether path is a directory from listing or selection
// metadata (SelectedDirPaths). known is false only when the path is not a current
// selection and is not in the listing — callers must not Stat on the UI goroutine.
func (s *State) knownSelectionIsDir(path string) (isDir bool, known bool) {
	path = cleanPathString(path)
	if path == "" {
		return false, false
	}
	if e, ok := s.listingEntry(path); ok {
		return e.Type == localfs.EntryDirectory, true
	}
	if s.SelectedDirPaths != nil && s.SelectedDirPaths[path] {
		return true, true
	}
	if s.SelectedPaths != nil && s.SelectedPaths[path] {
		return false, true
	}
	return false, false
}

// StripNavTarget returns the directory latched sync mirrors for a selected strip path and, when
// the path is a file, the basename to highlight. Type comes from listing or
// SelectedDirPaths; a selected path that is not marked as a directory is treated
// as a file. No filepath.Dir / os.Stat — sftp:// URIs stay backend-neutral.
func (s *State) StripNavTarget(raw string) (dir pathloc.Path, selectName string, err error) {
	loc, err := pathloc.Parse(raw)
	if err != nil {
		return pathloc.Path{}, "", err
	}
	isDir, known := s.knownSelectionIsDir(loc.String())
	if !known {
		isDir = false
	}
	if isDir {
		return loc, "", nil
	}
	parent := loc.Up()
	if parent.Equal(loc) || parent.IsZero() {
		return pathloc.Path{}, "", errors.New("cannot open parent of root")
	}
	return parent, SelectName(parent, loc.String(), loc.Base()), nil
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

// buildSubtreeSelAncestors counts, per directory, the selected paths strictly below it.
func buildSubtreeSelAncestors(selected map[string]bool) map[string]int32 {
	if len(selected) == 0 {
		return nil
	}
	out := make(map[string]int32)
	for p, on := range selected {
		if on {
			countSubtreeAncestors(out, p, 1)
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
	// Ask isDir (a stat for an unlisted path) once per batch path; the conflict checks ask about
	// existing selections too, which SelectedDirPaths already answers.
	batchDir := make(map[string]bool, len(paths))
	for _, p := range paths {
		if p = cleanPathString(p); p != "" {
			if _, ok := batchDir[p]; !ok {
				batchDir[p] = isDir(p)
			}
		}
	}
	known := s.SelectedDirPaths
	dirOf := func(p string) bool {
		if d, ok := batchDir[p]; ok {
			return d
		}
		return known[p]
	}

	removed := BulkApplySelectionAdds(s.SelectedPaths, paths, dirOf)
	// Patch the directory marks instead of rebuilding them: a rebuild stats every unlisted
	// selection, not just the batch.
	for p := range s.SelectedDirPaths {
		if !s.SelectedPaths[p] {
			s.unmarkSelectedDir(p)
		}
	}
	for p, d := range batchDir {
		if d && s.SelectedPaths[p] {
			s.markSelectedDir(p)
		}
	}
	s.recomputeSelectionListedBytes()
	s.invalidateSelectionDerived()
	return removed
}

// dropSelectedPath removes one selected path's per-path bookkeeping: the selection itself, its
// listed bytes and its directory mark. Callers own derived-state invalidation.
func (s *State) dropSelectedPath(p string) {
	delete(s.SelectedPaths, p)
	s.adjustSelectionListedBytes(p, false)
	s.unmarkSelectedDir(p)
}

// InvertSelections flips each of paths: the selected ones are unselected and the rest selected
// (isDir answers for those), each side in one bulk change. It reports whether the adds removed
// conflicting selections.
func (s *State) InvertSelections(paths []string, isDir func(string) bool) bool {
	var on, off []string
	for _, p := range paths {
		if s.IsSelectedPath(p) {
			on = append(on, p)
		} else {
			off = append(off, p)
		}
	}
	// Adds go first: the remove settles the strip cursor, which builds the derived state once
	// for both halves.
	conflicts := s.BulkAddSelections(off, isDir)
	s.RemoveSelections(on)
	return conflicts
}

// RemoveSelections unselects every selected path in paths and reports how many were selected.
// Per-path bookkeeping is O(1) with no stat. A batch that is small next to the selection patches
// the derived state path by path (O(depth) plus an O(n) strip splice each); a larger one rebuilds
// it once instead of once per path.
func (s *State) RemoveSelections(paths []string) int {
	patch := removeSelectionsPatches(len(paths), len(s.SelectedPaths))
	removed := 0
	for _, p := range paths {
		p = cleanPathString(p)
		if p == "" || !s.SelectedPaths[p] {
			continue
		}
		if patch {
			s.applySelectionRemove(p)
		} else {
			s.dropSelectedPath(p)
		}
		removed++
	}
	if removed == 0 {
		return 0
	}
	if !patch {
		if len(s.SelectedPaths) == 0 {
			s.clearSelectionState()
		}
		s.invalidateSelectionDerived()
	}
	s.normalizeSelectedStripCursor()
	return removed
}

// removeSelectionsPatches reports whether removing k of n selections is cheaper path by path
// than by one rebuild.
// ponytail: fixed crossover (a rebuild costs about what patching 1/100 of the selection does at
// 10k); retune with a k-of-n RemoveSelections benchmark if the numbers move.
func removeSelectionsPatches(k, n int) bool { return k*64 <= n }
