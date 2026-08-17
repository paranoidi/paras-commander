package panel

import (
	"context"
	"maps"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/treeflat"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/ui/lineedit"
)

const maxNavHistory = 200

// noIndexCursorFallback is passed to load as indexFallback to keep cursor at 0 when the
// preserve-by-name step cannot resolve (directory changes, initial load).
const noIndexCursorFallback = -1

// historyCursorSnapshot stores the highlighted row when leaving a directory so re-entry can restore it.
type historyCursorSnapshot struct {
	EntryName string
	Index     int
	// CursorPath is the highlighted entry's absolute path, used to disambiguate tree-mode
	// recall where Index/EntryName alone can't identify a nested descendant row.
	CursorPath string
	// TreeExpanded is the expanded node-ID set when leaving in tree mode; nil otherwise.
	TreeExpanded map[string]bool
	// TreeExpandAllDepth is the ExpandAllTreeShallow deepen counter when leaving in tree mode.
	TreeExpandAllDepth int
}

// State contains all panel data needed by the App and renderer.
type State struct {
	Path pathloc.Path
	// VolumeAvailBytes / VolumeTotalBytes describe the file system backing Path after the last successful load.
	VolumeSpaceOK    bool
	VolumeAvailBytes uint64
	VolumeTotalBytes uint64
	// ListingDevice is st_dev for Path after the last successful directory load (Unix). Valid false when unknown (e.g. Windows or Stat error).
	ListingDevice      uint64
	ListingDeviceValid bool
	Entries            []localfs.Entry
	Cursor             int
	ScrollOffset       int
	// History lists visited directories (most recent first). HistoryIndex is the timeline offset:
	// 0 = newest/current slot in the list (matches Path after navigation).
	History      []string
	HistoryIndex int
	// HistoryCursorByPath maps canonical directory paths to the last highlighted entry when leaving.
	HistoryCursorByPath map[string]historyCursorSnapshot
	SelectedPaths       map[string]bool
	// SelectedDirPaths tracks selected directory paths for O(1) selectionHasDirs and fast conflict checks.
	SelectedDirPaths map[string]bool
	// listingByPath maps entry.Path to listing index; rebuilt when Entries change.
	listingByPath map[string]localfs.Entry
	// selectionListedBytes sums file sizes for selected paths present in the current listing.
	selectionListedBytes int64
	// selectionDerivedGen bumps on selection mutations; selDerivedCache is rebuilt lazily per cwd.
	selectionDerivedGen uint64
	selDerivedCache     selectionDerivedCache
	// selectionHasDirs is true when any selected path is a directory (avoids stat scans on bulk file sets).
	selectionHasDirs bool
	// SelectionsStripOrder lists selected paths that belong in the bottom “selections” strip:
	// off-current-directory order is tracked here; on chdir into dir D those paths are removed;
	// on chdir away from D, selected paths under D are appended (deduped).
	SelectionsStripOrder  []string
	SelectionsStripCursor int
	SelectionsStripScroll int
	// SelectionStashPaths holds stashed selection paths (empty/nil = no stash).
	SelectionStashPaths []string
	// SelectionStashStripOrder mirrors SelectionsStripOrder at stash time.
	SelectionStashStripOrder []string
	ShowHidden               bool
	// Gitignore is a shared cache for .gitignore filtering when ShowHidden is false; nil disables.
	Gitignore *gitignore.Cache
	// GitignoreActive is true when the current listing applies Git ignore rules (inside a work tree).
	GitignoreActive bool
	// DotfilesHiddenActive is true when dotfiles are hidden and the current directory has dot-prefixed names.
	DotfilesHiddenActive bool
	// entriesShowHidden is the ShowHidden value in effect when Entries was last populated. Used to
	// skip the newly-appeared-file diff when a same-dir reload is really a visibility filter toggle
	// (SetShowHidden) rather than external filesystem changes — otherwise files that were always on
	// disk but newly revealed by the filter would get marked as new.
	entriesShowHidden bool
	// GitColumnActive is true when the listing path is inside a Git work tree with valid metadata (local panels only).
	GitColumnActive bool
	// GitPending is true while async git status is in flight for this listing.
	GitPending bool
	// GitByPath maps absolute entry paths to eza-style staged/unstaged cells; nil until loaded.
	GitByPath map[string]gitstatus.Cell
	// gitWorkRoot caches the work-tree root prepareGitColumn resolved for the current cwd
	// listing (valid only while GitColumnActive), so tree-mode child-directory git-status
	// fetches (scheduleTreeChildGitStatus) can reuse it instead of recomputing
	// gitignore.ValidWorkTreeRoot per subdirectory.
	gitWorkRoot string
	// gitStatusChildPending counts tree-child git-status fetches dispatched by
	// scheduleTreeChildGitStatus that haven't completed yet — see NoteTreeChildGitStatusApplied.
	gitStatusChildPending int
	Filter                FilterState
	// StripFilter is the selections-strip quick filter (basename fuzzy match), independent of Filter.
	StripFilter FilterState
	// ActiveEntryFilter narrows visible entries (e.g. git-status filtering); nil means unfiltered.
	ActiveEntryFilter *EntryFilter
	// filteredIdx holds raw (unfiltered) entry indices matching ActiveEntryFilter, in display order.
	filteredIdx []int
	// filteredTreeShape holds recomputed tree-connector shapes (LastChild/AncestorHasNext),
	// parallel to filteredIdx, for tree mode with a filter active — see
	// recomputeFilteredTreeConnectors. nil outside tree mode or when no filter is active.
	filteredTreeShape []treeConnectorShape
	// DiskSorter returns cached subtree or file aggregates for Disk usage sorting; absent cache ranks last until known.
	DiskSorter func(absPath string) (int64, bool)
	Sort       SortState
	// ListFormat controls trailing columns after size (Modified / Permissions / none). Per-panel; see config default_listing_format.
	ListFormat ListFormat
	// ScrollMode mirrors [ui.scroll].mode: minimal, center, or edge scroll policy.
	ScrollMode ScrollMode
	// ScrollEdgeMargin mirrors [ui.scroll].edge_margin for edge mode.
	ScrollEdgeMargin int
	// CarouselMode shows a three-column parent | current | child preview inside this panel.
	CarouselMode bool
	// CarouselChildPreviewCoalesce skips child-directory reads during scroll and reuses CarouselSideCache.Child.
	CarouselChildPreviewCoalesce bool
	// CursorNameHintCoalesce holds the previous bottom-border full-name overlay during file-list
	// nav debounce (paint uses CursorNameHintPinned until the debounce timer fires).
	CursorNameHintCoalesce bool
	// CursorNameHintPinned is the last full-name overlay text painted when not coalescing.
	// During CursorNameHintCoalesce it keeps the prior name visible until debounce settles.
	CursorNameHintPinned string
	// CarouselSideCache holds the last-built parent/child listing snapshots for carousel side columns.
	CarouselSideCache struct {
		Parent   ListingSnapshot
		ParentOK bool
		Child    ListingSnapshot
		ChildOK  bool
		// ChildCursorDir is the absolute path of the directory entry under the center cursor when
		// Child was cached; used to reject stale previews during nav coalesce.
		ChildCursorDir string
	}
	// IdleDiskTotalsSort is set after a user-initiated disk-usage analysis completes and idle-sort delay elapses
	// (DiskUsageIdleSizeSort). Selection-size background scans must not set this.
	IdleDiskTotalsSort bool
	// DiskUsageIdleSortEligible is set by the app when DiskUsageShown is true (user started disk-usage analysis).
	// Without it, a fully populated size cache (e.g. from selection-size scans) must not activate IdleDiskTotalsSort.
	DiskUsageIdleSortEligible bool
	// DiskUsageIdleSortActivated mirrors the disk-usage sort checkbox lifecycle (config/dialog apply).
	// Idle-sort scheduling keys off Sort.DiskUsageIdleSizeSort; this flag stays in sync for UI/state parity.
	DiskUsageIdleSortActivated bool

	// NewFileMarksByDir maps listing directory paths to latest/previous new-file marks
	// after completed copy/move/flatten into that directory. Marks are dropped when leaving the directory.
	NewFileMarksByDir map[string]*dirNewFileMarks

	// RenameMarksByDir maps listing directory paths to sets of base names that were renamed
	// in that directory. Marks are dropped when leaving the directory.
	RenameMarksByDir map[string]map[string]struct{}

	// OnDirectoryChange is called after every successful directory load (Enter, Parent,
	// HistoryBackward/Forward, Refresh, SetShowHidden, etc.). The app uses this to check whether disk-usage idle sorting
	// can be applied immediately or needs to be deferred.
	OnDirectoryChange func()
	// FileListViewportRows, when set by the app, returns the live file-list viewport row count
	// (after selections-strip layout for the current path). ApplyListing uses it when applying scroll
	// so Parent/Enter stay aligned with paint even when strip height changes on chdir.
	FileListViewportRows func() int
	// SuppressHeavyPathProbes, when set, skips statfs and device lookup in load() for paths
	// where those syscalls would contend with active background job I/O on the same volume.
	SuppressHeavyPathProbes func(pathloc.Path) bool
	// ScheduleAsyncLoad runs a directory listing off the UI thread (set by app; nil = synchronous,
	// used by tests that construct a bare State). Not remote-only: a local path can be just as
	// slow as a remote one (network mount, autofs trigger), and Go cannot cancel a goroutine
	// blocked inside a real blocking syscall — running it off-thread is what keeps the UI
	// responsive even when the underlying Stat/ReadDir never returns.
	ScheduleAsyncLoad AsyncLoadScheduler
	// ListingPending is true while an asynchronous listing is in flight.
	ListingPending bool
	// ScheduleGitStatus runs git status for the current listing off the UI thread (set by app).
	ScheduleGitStatus GitStatusScheduler
	// ScheduleTreeChildLoad runs a tree-mode directory's first-expand child listing off the UI
	// thread (set by app; nil = synchronous fallback, see setTreeNodeExpanded).
	ScheduleTreeChildLoad TreeChildLoadScheduler

	// ListLayout selects flat rows (default) or an expand/collapse tree for this panel's file list.
	ListLayout ListLayout
	// TreeRoots holds the tree built for tree-layout rendering. Depth-0 nodes mirror Entries;
	// deeper nodes are loaded lazily on first expand (see ToggleTreeExpand).
	TreeRoots []treeflat.Node[TreeEntry]
	// TreeExpanded tracks expand state by node ID (absolute path); default collapsed, so a path
	// absent from the map means collapsed.
	TreeExpanded map[string]bool
	// treeRows caches the last treeflat.Flatten(TreeRoots, ...) output.
	treeRows []treeflat.Row[TreeEntry]
	// treeCursorID tracks the node ID the cursor should reattach to after a tree rebuild
	// (expand/collapse, or an async ApplyTreeChildLoad completing).
	treeCursorID string
	// treeExpandQuiet is the number of in-flight async child loads coalesced by
	// ExpandAllTreeShallow: ApplyTreeChildLoad updates node state but skips rebuild/redraw
	// until this counter reaches zero, then reattaches the cursor to treeCursorID once.
	treeExpandQuiet int
	// treeExpandAllDepth is how many successful ExpandAllTreeShallow presses have deepened
	// this panel's tree (0 = none yet). Caps at MaxExpandAllShallowDepth; CollapseAllTree
	// decrements it one level at a time. Snapshotted per directory in HistoryCursorByPath and
	// restored on return; reset when re-entering tree mode.
	treeExpandAllDepth int
	// treeExpandAllAuto is true while an ExpandAllTreeFully cascade is in progress: each level's
	// async loads (see treeExpandQuiet) complete on their own schedule, and
	// finishTreeChildLoadApply re-drives the next level automatically until MaxExpandAllShallowDepth
	// is reached. Reset alongside treeExpandAllDepth wherever that gets reset, so a cascade orphaned
	// by mid-flight navigation can never misfire on a later, unrelated single-row expand.
	treeExpandAllAuto bool
	// treeCollapseGen bumps on every CollapseAllTree/CollapseAllTreeFully call. Each tree node's
	// async child-load dispatch captures the generation at dispatch time (TreeEntry.LoadGen);
	// ApplyTreeChildLoad compares it against the current value to drop stragglers — fetches
	// dispatched before the user's last whole-tree collapse, landing after it — instead of letting
	// them silently re-expand a directory the user just asked to collapse.
	treeCollapseGen int
}

// GitStatusRequest describes one async git status fetch for the current listing.
type GitStatusRequest struct {
	WorkRoot string
	ListDir  string
	Paths    []gitstatus.ListingPaths
}

// GitStatusScheduler returns true when a background git status fetch was started.
type GitStatusScheduler func(req GitStatusRequest) bool

// PathString returns the canonical path string (history, status bar, host APIs).
func (s *State) PathString() string {
	return s.Path.String()
}

// FilterState tracks panel-local quick filter state.
type FilterState struct {
	Query           string
	Cursor          int // rune offset within Query where typed/deleted runes apply
	Active          bool
	Editing         bool
	CaseInsensitive bool
	// CycleMatches is "visual" (default) or "ranked"; empty means visual.
	// It controls Up/Down traversal among quick-filter matches.
	CycleMatches string
	results      []filterResult
}

type filterResult struct {
	Index  int
	Score  int
	Ranges []search.Range
}

// New loads a panel rooted at path.
func New(path string) (State, error) {
	return NewWithOptions(path, localfs.DefaultListOptions(), nil)
}

// NewWithOptions loads a panel rooted at path with configured listing defaults.
// gitignoreCache is optional; when set it is used for the initial listing (and later reloads).
func NewWithOptions(path string, opts localfs.ListOptions, gitignoreCache *gitignore.Cache) (State, error) {
	state := State{
		Cursor:       0,
		ScrollOffset: 0,
		ShowHidden:   opts.ShowHidden,
		Gitignore:    gitignoreCache,
		Filter: FilterState{
			CaseInsensitive: true,
		},
		Sort: SortState{
			Mode:             SortName,
			Reverse:          false,
			DirectoriesFirst: true,
		},
	}
	if err := state.Load(path); err != nil {
		return State{}, err
	}
	return state, nil
}

// Load replaces the panel contents with a fresh directory snapshot.
func (s *State) Load(path string) error {
	return s.loadPathString(path, "", 0, noIndexCursorFallback, asyncLoadOpts{})
}

// Refresh reloads the current path. When the entry under the cursor still exists, it is re-selected by name;
// otherwise the prior row index is restored (clamped), matching MC-style behavior after moves/deletes.
// A no-op while a load is already in flight (ListingPending): Path still holds the pre-load
// directory until that load lands, so scheduling a second one here would race it on the shared
// generation counter and could clobber a real navigation with a stale-directory reload — the
// in-flight load already brings fresh contents for wherever it lands.
func (s *State) Refresh(viewportRows int) error {
	if s.ListingPending {
		return nil
	}
	priorCursor := s.Cursor
	entry, ok := s.CurrentEntry()
	selectedName := ""
	if ok {
		selectedName = entry.Name
	}
	return s.load(s.Path, selectedName, viewportRows, priorCursor, asyncLoadOpts{})
}

// RefreshOrNavigateToExistingAncestor reloads the current directory when it still exists.
// When the current path is missing, it walks up to the nearest existing ancestor and navigates
// there in one step (highlighting the first missing child name when possible).
func (s *State) RefreshOrNavigateToExistingAncestor(viewportRows int) error {
	if s.Path.IsZero() || DirectoryExists(s.Path) {
		return s.Refresh(viewportRows)
	}
	current := s.Path
	for {
		parent := current.Parent()
		if parent.Equal(current) {
			return s.Refresh(viewportRows)
		}
		if DirectoryExists(parent) {
			return s.NavigateToPath(parent, current.Base(), viewportRows)
		}
		current = parent
	}
}

// ApplyPeriodicRefresh commits a same-directory listing when content changed.
// Selection is restored by name (else prior index). Scroll centers when the restore would move the viewport.
func (s *State) ApplyPeriodicRefresh(listingLoc pathloc.Path, backendEntries []fsbackend.Entry, viewportRows int) (bool, error) {
	if fsbackend.EntriesListingEqual(backendEntries, BackendEntriesFromPanel(s.Entries)) {
		return false, nil
	}
	priorCursor := s.Cursor
	entry, ok := s.CurrentEntry()
	selectedName := ""
	if ok {
		selectedName = entry.Name
	}
	if err := s.ApplyListing(listingLoc, backendEntries, selectedName, viewportRows, priorCursor, false); err != nil {
		return false, err
	}
	return true, nil
}

// SetShowHidden applies hidden-file visibility and reloads the current directory using the
// same cursor rules as Refresh. No-op when the value is unchanged. Visibility is a global
// app setting; the app assigns the same value to every panel so they can never diverge.
func (s *State) SetShowHidden(shown bool, viewportRows int) error {
	if s.ShowHidden == shown {
		return nil
	}
	priorCursor := s.Cursor
	entry, ok := s.CurrentEntry()
	selectedName := ""
	if ok {
		selectedName = entry.Name
	}
	s.ShowHidden = shown
	return s.load(s.Path, selectedName, viewportRows, priorCursor, asyncLoadOpts{})
}

// Move changes the cursor by delta and keeps it visible.
func (s *State) Move(delta int, viewportRows int) {
	s.Cursor += delta
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// Page moves the cursor by a viewport-sized delta.
func (s *State) Page(delta int, viewportRows int) {
	rows := viewportRows
	if rows < 1 {
		rows = 1
	}
	s.Move(delta*rows, viewportRows)
}

// Top moves the cursor to the first entry.
func (s *State) Top(viewportRows int) {
	s.Cursor = 0
	s.EnsureCursorInViewport(viewportRows)
}

// Bottom moves the cursor to the last entry.
func (s *State) Bottom(viewportRows int) {
	if s.VisibleEntryCount() == 0 {
		s.Cursor = 0
		s.ScrollOffset = 0
		return
	}
	s.Cursor = s.VisibleEntryCount() - 1
	s.EnsureCursorInViewport(viewportRows)
}

// CurrentEntry returns the entry under the cursor.
func (s State) CurrentEntry() (localfs.Entry, bool) {
	entry, _, ok := s.VisibleEntry(s.Cursor)
	return entry, ok
}

// VisibleEntryCount returns the number of entries currently visible in the panel.
func (s State) VisibleEntryCount() int {
	if s.ActiveEntryFilter != nil {
		return len(s.filteredIdx)
	}
	if s.ListLayout == ListLayoutTree {
		return len(s.treeRows)
	}
	return len(s.Entries)
}

// VisibleEntry returns the visible entry and its backing index (Entries index in flat mode,
// treeRows index in tree mode — every caller today discards this second value).
func (s State) VisibleEntry(index int) (localfs.Entry, int, bool) {
	rawIdx, ok := s.translateVisibleIndex(index)
	if !ok {
		return localfs.Entry{}, 0, false
	}
	if s.ListLayout == ListLayoutTree {
		return s.treeVisibleEntry(rawIdx)
	}
	return s.flatVisibleEntry(rawIdx)
}

// flatVisibleEntry is VisibleEntry's original body, extracted unchanged. Bounds-checks directly
// against len(s.Entries) rather than VisibleEntryCount(): index here is a raw Entries index (once
// an active filter has already remapped it through filteredIdx in VisibleEntry), not a filtered
// display index, so it must not be checked against the filtered count.
func (s State) flatVisibleEntry(index int) (localfs.Entry, int, bool) {
	if index < 0 || index >= len(s.Entries) {
		return localfs.Entry{}, 0, false
	}
	return s.Entries[index], index, true
}

func (s State) treeVisibleEntry(index int) (localfs.Entry, int, bool) {
	if index < 0 || index >= len(s.treeRows) {
		return localfs.Entry{}, 0, false
	}
	return s.treeRows[index].Value.Entry, index, true
}

// VisibleEntries returns the currently visible entries as a flat slice (Entries in flat mode,
// flattened treeRows in tree mode). For callers that need every visible row's Entry but not the
// per-row tree shape (e.g. the disk-usage bar denominator, which only needs byte sizes).
func (s State) VisibleEntries() []localfs.Entry {
	count := s.VisibleEntryCount()
	if count == 0 {
		return nil
	}
	out := make([]localfs.Entry, 0, count)
	for i := 0; i < count; i++ {
		entry, _, ok := s.VisibleEntry(i)
		if !ok {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// FilterHasMatches reports whether the active quick filter has at least one file-name match.
func (s State) FilterHasMatches() bool {
	return len(s.Filter.results) > 0
}

// FilterUniqueMatch reports whether the active quick filter has exactly one file-name match.
func (s State) FilterUniqueMatch() bool {
	return s.Filter.Active && len(s.Filter.results) == 1
}

// MatchRanges returns highlighted rune ranges for the visible entry.
func (s State) MatchRanges(index int) []search.Range {
	if !s.Filter.Active || index < 0 || index >= s.VisibleEntryCount() {
		return nil
	}
	for _, result := range s.Filter.results {
		if result.Index == index {
			return result.Ranges
		}
	}
	return nil
}

// AddSelection marks path as selected without changing the file-list cursor.
// Returns true if conflicting selections were removed before adding path.
func (s *State) AddSelection(path string) bool {
	path = cleanPathString(path)
	if path == "" {
		return false
	}
	if s.SelectedPaths != nil && s.SelectedPaths[path] {
		return false
	}
	isDir := s.selectedPathIsDirectory(path)
	conflicts := s.resolveSelectionConflicts(path, isDir)
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	s.applySelectionAdd(path, isDir)
	return conflicts
}

// TogglePathSelection toggles selection for an absolute path not required to be listed.
func (s *State) TogglePathSelection(path string) (selected bool, conflictsRemoved bool) {
	path = cleanPathString(path)
	if path == "" {
		return false, false
	}
	isDir := s.selectedPathIsDirectory(path)
	if s.SelectedPaths != nil && s.SelectedPaths[path] {
		s.applySelectionRemove(path, isDir)
		s.removePathFromSelectionsStripOrder(path)
		s.normalizeSelectionsStripCursor()
		return false, false
	}
	conflictsRemoved = s.resolveSelectionConflicts(path, isDir)
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	s.applySelectionAdd(path, isDir)
	return true, conflictsRemoved
}

// ToggleSelection toggles the current entry in the panel-local selection set.
// The first bool is true when the entry is selected after the call; the second reports conflict removals.
func (s *State) ToggleSelection() (selected bool, conflictsRemoved bool) {
	entry, ok := s.CurrentEntry()
	if !ok {
		return false, false
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	if s.SelectedPaths[entry.Path] {
		wasDir := entry.IsDir()
		s.applySelectionRemove(entry.Path, wasDir)
		s.removePathFromSelectionsStripOrder(entry.Path)
		s.normalizeSelectionsStripCursor()
		return false, false
	}
	conflictsRemoved = s.resolveSelectionConflicts(entry.Path, entry.IsDir())
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	s.applySelectionAdd(entry.Path, entry.IsDir())
	return true, conflictsRemoved
}

// ToggleSelectionAndAdvance toggles the current entry, then moves to the next row when possible.
// The first bool is true when toggle ran; the second reports conflict removals.
func (s *State) ToggleSelectionAndAdvance(viewportRows int) (bool, bool) {
	if _, ok := s.CurrentEntry(); !ok {
		return false, false
	}
	_, conflictsRemoved := s.ToggleSelection()
	if s.Cursor < s.VisibleEntryCount()-1 {
		s.Move(1, viewportRows)
	}
	return true, conflictsRemoved
}

// IsSelected reports whether entry is selected in this panel.
func (s State) IsSelected(entry localfs.Entry) bool {
	return s.IsSelectedPath(entry.Path)
}

// IsSelectedPath reports whether an absolute path is selected.
func (s State) IsSelectedPath(path string) bool {
	path = cleanPathString(path)
	return path != "" && s.SelectedPaths != nil && s.SelectedPaths[path]
}

// HasSelectionInSubtree reports whether some selected path is a strict descendant of dirPath.
func (s *State) HasSelectionInSubtree(dirPath string) bool {
	if s.SelectedPaths == nil {
		return false
	}
	dir := cleanPathString(dirPath)
	if dir == "" {
		return false
	}
	s.ensureSelectionDerived()
	return s.selDerivedCache.subtreeAncestors[dir]
}

// IsStrictPathDescendant reports whether child is a strict descendant of parent (different paths, child under parent).
func IsStrictPathDescendant(parent, child string) bool {
	return isStrictPathDescendant(cleanPathString(parent), cleanPathString(child))
}

func isStrictPathDescendant(parent, child string) bool {
	p, err1 := pathloc.Parse(parent)
	c, err2 := pathloc.Parse(child)
	if err1 != nil || err2 != nil || p.Scheme() != c.Scheme() {
		return false
	}
	if p.Equal(c) {
		return false
	}
	return c.HasPrefix(p)
}

// SelectedDirectoryPaths returns absolute paths of selected directories in stable sorted order.
func (s State) SelectedDirectoryPaths() []string {
	entries, _ := s.SelectedEntries(true, localfs.EntryFromPath)
	if len(entries) == 0 {
		return nil
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Type == localfs.EntryDirectory {
			out = append(out, e.Path)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// PruneNestedPaths drops paths that are strict descendants of another path in the slice.
// O(n log n) via sorted single-pass; out maintains no nested pairs.
func PruneNestedPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	if len(paths) == 1 {
		if p := cleanPathString(paths[0]); p != "" {
			return []string{p}
		}
		return nil
	}
	sorted := make([]string, 0, len(paths))
	for _, p := range paths {
		if p = cleanPathString(p); p != "" {
			sorted = append(sorted, p)
		}
	}
	if len(sorted) == 0 {
		return nil
	}
	sort.Strings(sorted)
	out := make([]string, 0, len(sorted))
	for _, p := range sorted {
		if len(out) > 0 && isStrictPathDescendant(out[len(out)-1], p) {
			continue
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Enter opens the selected directory. Regular files are intentionally inert.
func (s *State) Enter(viewportRows int) (bool, error) {
	entry, ok := s.CurrentEntry()
	if !ok || entry.Type != localfs.EntryDirectory {
		return false, nil
	}
	if err := s.NavigateTo(entry.Path, "", viewportRows); err != nil {
		return false, err
	}
	return true, nil
}

// Parent opens the parent directory and selects the directory just exited.
func (s *State) Parent(viewportRows int) error {
	if s.Path.IsZero() {
		return nil
	}
	current := s.Path
	parent := current.Parent()
	selectedName := current.Base()
	if parent.Equal(current) {
		selectedName = ""
	}
	return s.NavigateToPath(parent, selectedName, viewportRows)
}

// NavigateTo loads path after recording it in navigation history (MRU timeline).
func (s *State) NavigateTo(path string, selectedName string, viewportRows int) error {
	loc, err := pathloc.Parse(path)
	if err != nil {
		return err
	}
	return s.NavigateToPath(loc, selectedName, viewportRows)
}

// NavigateToPath loads loc after recording it in navigation history (MRU timeline).
func (s *State) NavigateToPath(loc pathloc.Path, selectedName string, viewportRows int) error {
	target := loc.String()
	s.recordVisit(target)
	if err := s.load(loc, selectedName, viewportRows, noIndexCursorFallback, asyncLoadOpts{
		rollback:        func() { s.revertRecordedVisit(target) },
		syncHistoryHead: true,
	}); err != nil {
		return err
	}
	if !s.ListingPending && s.HistoryIndex == 0 && len(s.History) > 0 {
		s.History[0] = cleanPathString(s.Path.String())
	}
	return nil
}

// HistoryBackward moves to an older entry in the timeline (larger HistoryIndex).
func (s *State) HistoryBackward(viewportRows int) (bool, error) {
	if len(s.History) == 0 || s.HistoryIndex >= len(s.History)-1 {
		return false, nil
	}
	nextIdx := s.HistoryIndex + 1
	target := cleanPathString(s.History[nextIdx])
	if target == "" {
		return false, nil
	}
	prevIdx := s.HistoryIndex
	s.HistoryIndex = nextIdx
	if err := s.loadPathString(target, "", viewportRows, noIndexCursorFallback, asyncLoadOpts{
		rollback: func() { s.HistoryIndex = prevIdx },
	}); err != nil {
		s.HistoryIndex = prevIdx
		return false, err
	}
	if s.ListingPending {
		return true, nil
	}
	if s.HistoryIndex >= 0 && s.HistoryIndex < len(s.History) {
		s.History[s.HistoryIndex] = cleanPathString(s.Path.String())
	}
	return true, nil
}

// HistoryForward moves to a newer entry in the timeline (smaller HistoryIndex).
func (s *State) HistoryForward(viewportRows int) (bool, error) {
	if s.HistoryIndex <= 0 {
		return false, nil
	}
	nextIdx := s.HistoryIndex - 1
	target := cleanPathString(s.History[nextIdx])
	prevIdx := s.HistoryIndex
	s.HistoryIndex = nextIdx
	if err := s.loadPathString(target, "", viewportRows, noIndexCursorFallback, asyncLoadOpts{
		rollback: func() { s.HistoryIndex = prevIdx },
	}); err != nil {
		s.HistoryIndex = prevIdx
		return false, err
	}
	if s.ListingPending {
		return true, nil
	}
	if s.HistoryIndex >= 0 && s.HistoryIndex < len(s.History) {
		s.History[s.HistoryIndex] = cleanPathString(s.Path.String())
	}
	return true, nil
}

func (s *State) recordVisit(target string) {
	target = cleanPathString(target)
	if target == "" {
		return
	}
	cur := cleanPathString(s.Path.String())
	if target == cur {
		return
	}
	var base []string
	if len(s.History) > 0 && s.HistoryIndex < len(s.History) {
		// Drop the forward branch when navigating after history-back/forward.
		base = append([]string(nil), s.History[s.HistoryIndex:]...)
	}
	base = removePathFromSlice(base, target)
	hist := append([]string{target}, base...)
	if len(hist) > maxNavHistory {
		hist = hist[:maxNavHistory]
	}
	s.History = hist
	s.HistoryIndex = 0
	s.pruneHistoryCursors()
}

func (s *State) rememberCursorForPath(dir string) {
	dir = cleanPathString(dir)
	if dir == "" {
		return
	}
	name := ""
	path := ""
	if entry, ok := s.CurrentEntry(); ok {
		name = entry.Name
		path = entry.Path
	}
	if s.HistoryCursorByPath == nil {
		s.HistoryCursorByPath = make(map[string]historyCursorSnapshot)
	}
	snap := historyCursorSnapshot{EntryName: name, Index: s.Cursor, CursorPath: path}
	if s.ListLayout == ListLayoutTree {
		snap.TreeExpanded = maps.Clone(s.TreeExpanded)
		snap.TreeExpandAllDepth = s.treeExpandAllDepth
	}
	s.HistoryCursorByPath[dir] = snap
}

func (s *State) recalledCursorFor(dir string) (selectedName string, indexFallback int, recalled bool) {
	dir = cleanPathString(dir)
	if dir == "" {
		return "", noIndexCursorFallback, false
	}
	snap, ok := s.HistoryCursorByPath[dir]
	if !ok {
		return "", noIndexCursorFallback, false
	}
	return snap.EntryName, snap.Index, true
}

// RecalledCursorFor returns the saved highlight for dir from this panel's visit history.
func (s State) RecalledCursorFor(dir string) (entryName string, index int, ok bool) {
	return s.recalledCursorFor(dir)
}

// CleanPathString canonicalizes a path string for history and recall map keys (pathloc form).
func CleanPathString(p string) string {
	return cleanPathString(p)
}

// MergeNavigationHistories combines passive-first then active navigation histories,
// deduplicating by cleaned path and skipping empty paths.
func MergeNavigationHistories(passive, active []string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		if p == "" || p == "." {
			return
		}
		cp := cleanPathString(p)
		if cp == "" {
			return
		}
		if _, ok := seen[cp]; ok {
			return
		}
		seen[cp] = struct{}{}
		out = append(out, cp)
	}
	for _, p := range passive {
		add(p)
	}
	for _, p := range active {
		add(p)
	}
	return out
}

// BestRecalledCursor returns the best saved highlight for dir across panels. A non-empty
// entry name wins over index-only snapshots (e.g. cursor left on ".." when exiting once).
func BestRecalledCursor(dir string, states ...*State) (entryName string, index int, ok bool) {
	dir = cleanPathString(dir)
	if dir == "" {
		return "", noIndexCursorFallback, false
	}
	var indexOnlyIdx int
	var indexOnly bool
	for _, st := range states {
		if st == nil {
			continue
		}
		n, i, found := st.recalledCursorFor(dir)
		if !found {
			continue
		}
		if n != "" {
			return n, i, true
		}
		if !indexOnly {
			indexOnlyIdx = i
			indexOnly = true
		}
	}
	if indexOnly {
		return "", indexOnlyIdx, true
	}
	return "", noIndexCursorFallback, false
}

// MergeHistoryCursorByPath merges cursor-recall maps. Entries in primary override secondary
// for the same directory key unless primary has an empty name and secondary does not.
func MergeHistoryCursorByPath(secondary, primary map[string]historyCursorSnapshot) map[string]historyCursorSnapshot {
	if len(secondary) == 0 && len(primary) == 0 {
		return nil
	}
	out := make(map[string]historyCursorSnapshot, len(secondary)+len(primary))
	for k, v := range secondary {
		out[k] = v
	}
	for k, v := range primary {
		if existing, exists := out[k]; exists && existing.EntryName != "" && v.EntryName == "" {
			continue
		}
		out[k] = v
	}
	return out
}

// NoIndexCursorFallback is the indexFallback value for load that means "use recall or row 0".
const NoIndexCursorFallback = noIndexCursorFallback

// LoadWithViewport loads path and restores selectedName, indexFallback, or HistoryCursorByPath recall.
func (s *State) LoadWithViewport(path string, selectedName string, viewportRows int, indexFallback int) error {
	return s.loadPathString(path, selectedName, viewportRows, indexFallback, asyncLoadOpts{})
}

func (s *State) resolveLoadCursor(loc pathloc.Path, selectedName string, indexFallback int) (string, int, bool) {
	if selectedName != "" || indexFallback != noIndexCursorFallback {
		return selectedName, indexFallback, false
	}
	sn, idx, ok := s.recalledCursorFor(loc.String())
	return sn, idx, ok
}

func (s *State) pruneHistoryCursors() {
	if len(s.HistoryCursorByPath) <= maxNavHistory {
		return
	}
	keep := make(map[string]struct{}, len(s.History))
	for _, p := range s.History {
		if cp := cleanPathString(p); cp != "" {
			keep[cp] = struct{}{}
		}
	}
	for dir := range s.HistoryCursorByPath {
		if _, ok := keep[dir]; !ok {
			delete(s.HistoryCursorByPath, dir)
		}
	}
}

func removePathFromSlice(slice []string, target string) []string {
	want := cleanPathString(target)
	if want == "" {
		return slice
	}
	out := slice[:0]
	for _, p := range slice {
		if cleanPathString(p) != want {
			out = append(out, p)
		}
	}
	return out
}

// shouldCenterCursorOnListing reports whether ApplyListing should center the cursor after
// restoring a highlight. Recall and explicit selections (e.g. Parent) center on chdir;
// same-directory reloads (Refresh, SetShowHidden) keep minimal scroll.
func shouldCenterCursorOnListing(previousPath, listingLoc pathloc.Path, centerRecalled bool, selectedName string, indexFallback int) bool {
	if previousPath.Equal(listingLoc) {
		return false
	}
	return centerRecalled || selectedName != "" || indexFallback >= 0
}

// cursorAppearsCentered reports whether the highlight row sits in the centered (or tail-pinned) viewport
// position produced by EnsureCursorCentered.
func (s *State) cursorAppearsCentered(viewportRows int) bool {
	n := s.VisibleEntryCount()
	if viewportRows <= 0 || n == 0 {
		return false
	}
	row := s.Cursor - s.ScrollOffset
	mid := viewportRows / 2
	if row == mid {
		return true
	}
	if n <= viewportRows {
		return row == 0
	}
	maxOffset := n - viewportRows
	return s.ScrollOffset == maxOffset && row == viewportRows-1
}

// cursorAppearsEdged reports whether the highlight row sits at the edge-margin viewport position.
func (s *State) cursorAppearsEdged(viewportRows int) bool {
	n := s.VisibleEntryCount()
	if viewportRows <= 0 || n == 0 || EffectiveScrollMode(s.ScrollMode) != ScrollModeEdge {
		return false
	}
	eff := geom.EffectiveEdgeMargin(viewportRows, s.ScrollEdgeMargin)
	pos := s.Cursor - s.ScrollOffset
	topMargin := pos
	bottomMargin := viewportRows - 1 - pos
	if n <= viewportRows {
		return true
	}
	maxOffset := n - viewportRows
	if s.ScrollOffset == 0 && topMargin < eff {
		return true
	}
	if s.ScrollOffset == maxOffset && bottomMargin < eff {
		return true
	}
	return topMargin >= eff && bottomMargin >= eff
}

// applyHighlightScroll is the single scroll policy after the highlight row is chosen (by name, path, or index).
// When centerOnHighlight is true, the row is centered when possible (Parent, rename recall, history re-entry).
// fallbackViewportRows may be stale (e.g. captured before chdir); FileListViewportRows is consulted when set.
func (s *State) applyHighlightScroll(fallbackViewportRows int, centerOnHighlight bool) {
	vr := s.effectiveFileListViewportRows(fallbackViewportRows)
	s.clampCursor()
	if centerOnHighlight {
		s.EnsureCursorCentered(vr)
		return
	}
	switch EffectiveScrollMode(s.ScrollMode) {
	case ScrollModeCenter:
		s.EnsureCursorCentered(vr)
	case ScrollModeEdge:
		s.EnsureCursorEdge(vr)
	default:
		s.EnsureCursorVisible(vr)
	}
}

func (s *State) effectiveFileListViewportRows(fallback int) int {
	if s.FileListViewportRows != nil {
		if vr := s.FileListViewportRows(); vr > 0 {
			return vr
		}
	}
	return fallback
}

// clampScrollKeepingCursorVisible returns a scroll offset that prefers preferredScroll while keeping cursor visible.
func clampScrollKeepingCursorVisible(preferredScroll, cursor, viewportRows, entryCount int) int {
	if viewportRows <= 0 || entryCount == 0 {
		return 0
	}
	scroll := preferredScroll
	maxOffset := entryCount - viewportRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if scroll > maxOffset {
		scroll = maxOffset
	}
	if scroll < 0 {
		scroll = 0
	}
	if cursor < scroll {
		scroll = cursor
	}
	if cursor >= scroll+viewportRows {
		scroll = cursor - viewportRows + 1
	}
	return scroll
}

// finishSameDirectoryReloadScroll applies scroll policy after a same-directory listing reload.
func (s *State) finishSameDirectoryReloadScroll(priorCursor, priorScroll, viewportRows int, wasCentered bool) {
	vr := s.effectiveFileListViewportRows(viewportRows)
	s.clampCursor()
	n := s.VisibleEntryCount()

	if s.ScrollMode == ScrollModeCenter {
		s.applyHighlightScroll(viewportRows, true)
		return
	}
	if s.Cursor == priorCursor {
		s.ScrollOffset = clampScrollKeepingCursorVisible(priorScroll, s.Cursor, vr, n)
		return
	}
	center := wasCentered || s.Cursor != priorCursor
	s.applyHighlightScroll(viewportRows, center)
}

// EnsureCursorVisible updates ScrollOffset so Cursor is in the viewport.
func (s *State) EnsureCursorVisible(viewportRows int) {
	s.clampCursor()
	if viewportRows <= 0 || s.VisibleEntryCount() == 0 {
		s.ScrollOffset = 0
		return
	}

	maxOffset := s.VisibleEntryCount() - viewportRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.ScrollOffset > maxOffset {
		s.ScrollOffset = maxOffset
	}
	if s.ScrollOffset < 0 {
		s.ScrollOffset = 0
	}
	if s.Cursor < s.ScrollOffset {
		s.ScrollOffset = s.Cursor
	}
	if s.Cursor >= s.ScrollOffset+viewportRows {
		s.ScrollOffset = s.Cursor - viewportRows + 1
	}
}

// EnsureCursorCentered sets ScrollOffset so the cursor row is near the middle of the viewport.
func (s *State) EnsureCursorCentered(viewportRows int) {
	s.clampCursor()
	n := s.VisibleEntryCount()
	if viewportRows <= 0 || n == 0 {
		s.ScrollOffset = 0
		return
	}
	if n <= viewportRows {
		s.ScrollOffset = 0
		return
	}
	maxOffset := n - viewportRows
	half := viewportRows / 2
	target := s.Cursor - half
	if target < 0 {
		target = 0
	}
	if target > maxOffset {
		target = maxOffset
	}
	s.ScrollOffset = target
}

// EnsureCursorEdge updates ScrollOffset only when the cursor is within [ui.scroll].edge_margin rows of the viewport edge.
func (s *State) EnsureCursorEdge(viewportRows int) {
	s.clampCursor()
	n := s.VisibleEntryCount()
	if viewportRows <= 0 || n == 0 {
		s.ScrollOffset = 0
		return
	}
	s.ScrollOffset = geom.ScrollOffsetEdge(s.Cursor, s.ScrollOffset, viewportRows, n, s.ScrollEdgeMargin)
}

// EnsureCursorInViewport scrolls according to ScrollMode.
func (s *State) EnsureCursorInViewport(fallbackViewportRows int) {
	vr := s.effectiveFileListViewportRows(fallbackViewportRows)
	switch EffectiveScrollMode(s.ScrollMode) {
	case ScrollModeCenter:
		s.EnsureCursorCentered(vr)
	case ScrollModeEdge:
		s.EnsureCursorEdge(vr)
	default:
		s.EnsureCursorVisible(vr)
	}
}

// SetFilterCaseInsensitive configures the quick filter case behavior.
func (s *State) SetFilterCaseInsensitive(value bool, viewportRows int) {
	s.Filter.CaseInsensitive = value
	s.StripFilter.CaseInsensitive = value
	selectedName := s.currentEntryName()
	s.rebuildFilter()
	s.rebuildStripFilter()
	_ = s.SelectVisibleEntry(selectedName)
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// OpenFilter starts editing the panel-local quick filter.
func (s *State) OpenFilter(viewportRows int) {
	s.Filter.Editing = true
	s.rebuildFilter()
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// AcceptFilter exits editing while keeping the current filtered view.
func (s *State) AcceptFilter(viewportRows int) {
	s.Filter.Editing = false
	s.Filter.Active = s.Filter.Query != ""
	if !s.Filter.Active {
		s.Filter.results = nil
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// CancelFilter exits editing and restores the unfiltered panel view.
func (s *State) CancelFilter(viewportRows int) {
	s.Filter.Editing = false
	s.applyFilterQuery("", viewportRows)
}

// ClearFilter removes the query and restores all entries while preserving edit mode.
func (s *State) ClearFilter(viewportRows int) {
	editing := s.Filter.Editing
	s.applyFilterQuery("", viewportRows)
	s.Filter.Editing = editing
}

// AppendFilterRune inserts a printable rune at the caret.
func (s *State) AppendFilterRune(value rune, viewportRows int) {
	s.Filter.Editing = true
	runes := []rune(s.Filter.Query)
	pos := lineedit.ClampRuneCursor(s.Filter.Cursor, len(runes))
	next := make([]rune, 0, len(runes)+1)
	next = append(next, runes[:pos]...)
	next = append(next, value)
	next = append(next, runes[pos:]...)
	s.Filter.Cursor = pos + 1
	s.applyFilterQuery(string(next), viewportRows)
}

// BackspaceFilter removes the rune before the caret.
func (s *State) BackspaceFilter(viewportRows int) {
	runes := []rune(s.Filter.Query)
	if len(runes) == 0 {
		s.Filter.Editing = false
		return
	}
	pos := lineedit.ClampRuneCursor(s.Filter.Cursor, len(runes))
	if pos == 0 {
		return
	}
	s.Filter.Editing = true
	next := make([]rune, 0, len(runes)-1)
	next = append(next, runes[:pos-1]...)
	next = append(next, runes[pos:]...)
	s.Filter.Cursor = pos - 1
	s.applyFilterQuery(string(next), viewportRows)
}

// MoveFilterCursorHome moves the filter caret to the start of the query.
func (s *State) MoveFilterCursorHome() {
	s.Filter.Cursor = 0
}

// MoveFilterCursorEnd moves the filter caret to the end of the query.
func (s *State) MoveFilterCursorEnd() {
	s.Filter.Cursor = len([]rune(s.Filter.Query))
}

// CycleFilterMatch moves the cursor through fuzzy matches in an order controlled by
// Filter.CycleMatches ("visual" = row order, "ranked" = score order).
// If nothing matches the current filter query, delta is applied as a normal cursor step (see Move).
// Movement wraps at the first and last matched rows.
func (s *State) CycleFilterMatch(delta int, viewportRows int) {
	if len(s.Filter.results) == 0 {
		s.Move(delta, viewportRows)
		return
	}
	order := s.filterResultsCycleOrder()
	n := len(order)
	cur := -1
	for i := range order {
		if order[i].Index == s.Cursor {
			cur = i
			break
		}
	}
	if cur < 0 {
		if delta > 0 {
			cur = nextFilterMatchIndex(order, s.Cursor)
		} else {
			cur = previousFilterMatchIndex(order, s.Cursor)
		}
	} else {
		cur = (cur + delta) % n
		if cur < 0 {
			cur += n
		}
	}
	s.Cursor = order[cur].Index
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

func (s *State) filterResultsCycleOrder() []filterResult {
	if s.Filter.cycleMatchesRanked() {
		return s.Filter.results
	}
	out := make([]filterResult, len(s.Filter.results))
	copy(out, s.Filter.results)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Index < out[j].Index
	})
	return out
}

func (f FilterState) cycleMatchesRanked() bool {
	return strings.EqualFold(strings.TrimSpace(f.CycleMatches), "ranked")
}

func nextFilterMatchIndex(results []filterResult, cursor int) int {
	for i, result := range results {
		if result.Index > cursor {
			return i
		}
	}
	return 0
}

func previousFilterMatchIndex(results []filterResult, cursor int) int {
	for i := len(results) - 1; i >= 0; i-- {
		if results[i].Index < cursor {
			return i
		}
	}
	return len(results) - 1
}

func (s *State) loadPathString(path string, selectedName string, viewportRows int, indexFallback int, remote asyncLoadOpts) error {
	loc, err := pathloc.Parse(path)
	if err != nil {
		return err
	}
	return s.load(loc, selectedName, viewportRows, indexFallback, remote)
}

func (s *State) load(loc pathloc.Path, selectedName string, viewportRows int, indexFallback int, remote asyncLoadOpts) error {
	if !loc.Equal(s.Path) {
		s.rememberCursorForPath(s.Path.String())
		s.dropNewFileMarks(s.Path.String())
		s.dropRenameMarks(s.Path.String())
	}
	selectedName, indexFallback, centerRecalled := s.resolveLoadCursor(loc, selectedName, indexFallback)
	if s.ScheduleAsyncLoad != nil {
		if s.ScheduleAsyncLoad(AsyncLoadRequest{
			Loc:                  loc,
			SelectedName:         selectedName,
			ViewportRows:         viewportRows,
			IndexFallback:        indexFallback,
			CenterRecalledCursor: centerRecalled,
			Rollback:             remote.rollback,
			SyncHistoryHead:      remote.syncHistoryHead,
		}) {
			s.ListingPending = true
			return nil
		}
	}
	backendEntries, listingLoc, gitignoreActive, dotfilesHiddenActive, err := s.fetchBackendEntries(loc)
	if err != nil {
		return err
	}
	s.GitignoreActive = gitignoreActive
	s.DotfilesHiddenActive = dotfilesHiddenActive
	return s.ApplyListing(listingLoc, backendEntries, selectedName, viewportRows, indexFallback, centerRecalled)
}

func (s *State) fetchBackendEntries(loc pathloc.Path) ([]fsbackend.Entry, pathloc.Path, bool, bool, error) {
	return FetchListing(context.Background(), s.ListingRefreshSnapshot(loc, 0))
}

// DirectoryExists reports whether loc is an existing directory on the backing VFS.
func DirectoryExists(loc pathloc.Path) bool {
	if loc.IsZero() {
		return false
	}
	entry, err := fsbackend.Default().Stat(context.Background(), loc)
	if err != nil {
		return false
	}
	return entry.Type == fsbackend.EntryDirectory
}

// ApplyListing commits backend entries into panel state (used after sync or async remote list).
func (s *State) ApplyListing(listingLoc pathloc.Path, backendEntries []fsbackend.Entry, selectedName string, viewportRows int, indexFallback int, centerRecalled bool) error {
	localEntries, err := fsbackend.ToPanelEntries(backendEntries)
	if err != nil {
		return err
	}

	previousPath := s.Path
	sameDirReload := previousPath.Equal(listingLoc)
	priorCursor := s.Cursor
	priorTreeCursorID := ""
	if s.ListLayout == ListLayoutTree && sameDirReload && priorCursor >= 0 && priorCursor < len(s.treeRows) {
		priorTreeCursorID = s.treeRows[priorCursor].ID
	}
	priorScroll := s.ScrollOffset
	wasCentered := false
	if sameDirReload {
		wasCentered = s.cursorAppearsCentered(s.effectiveFileListViewportRows(viewportRows))
	}
	var newlyAppeared []string
	if sameDirReload && s.entriesShowHidden == s.ShowHidden {
		newlyAppeared = newlyAppearedNames(s.Entries, localEntries)
	}
	s.entriesShowHidden = s.ShowHidden
	s.Path = listingLoc
	if listingLoc.IsRemote() {
		s.GitignoreActive = false
		s.GitColumnActive = false
		s.GitPending = false
		s.GitByPath = nil
		s.VolumeSpaceOK = false
		s.VolumeAvailBytes = 0
		s.VolumeTotalBytes = 0
		s.ListingDeviceValid = false
	} else if s.SuppressHeavyPathProbes == nil || !s.SuppressHeavyPathProbes(listingLoc) {
		s.refreshVolumeSpace(listingLoc)
		host := listingLoc.FilePathMust()
		dev, devOK := diskusage.PathDevice(host)
		s.ListingDevice = dev
		s.ListingDeviceValid = devOK
	} else {
		s.ListingDeviceValid = false
	}
	s.prepareGitColumn(listingLoc, localEntries)
	s.ClearFilterIfInapplicable()
	s.Entries = localEntries
	if len(newlyAppeared) > 0 {
		s.AddNewFileMarks(listingLoc, newlyAppeared)
	}
	s.rebuildListingByPath()
	s.recomputeSelectionListedBytes()
	s.Cursor = 0
	s.ScrollOffset = 0
	if !previousPath.Equal(listingLoc) {
		s.invalidateCarouselChildCache()
		s.notifyChdir(previousPath, listingLoc)
	}
	// Activate disk-total primary sort before ApplySort so the first paint matches MC-style
	// disk ordering when a user-initiated analysis has run and cache already covers this listing
	// (no idle timer / reconcile delay). Selection-size cache alone must not enable this.
	if s.Sort.DiskUsageIdleSizeSort && s.DiskUsageIdleSortEligible && len(s.Entries) > 0 && s.ListingFullyDiskCached() {
		s.DiskUsageIdleSortActivated = true
		s.IdleDiskTotalsSort = true
	}
	s.ApplySort()
	if s.ListLayout == ListLayoutTree {
		// TreeRoots is always re-rooted fresh from the just-fetched listing — cached subtree
		// content is never reused — but the *set* of which dirs were expanded carries over: from
		// the live in-memory state on a same-directory refresh, or from the per-path recall
		// snapshot on navigation. restoreTreeExpansions re-fetches each remembered dir's children
		// (below, after cursor selection needs treeRows populated). Reseeded after ApplySort (not
		// before) so depth-0 tree rows reflect the panel's sort setting instead of raw backend
		// order.
		var keep map[string]bool
		if sameDirReload {
			keep = s.TreeExpanded
		} else if snap, ok := s.HistoryCursorByPath[cleanPathString(listingLoc.String())]; ok {
			keep = maps.Clone(snap.TreeExpanded)
			s.treeExpandAllDepth = snap.TreeExpandAllDepth
			s.treeExpandAllAuto = false
		} else {
			s.treeExpandAllDepth = 0
			s.treeExpandAllAuto = false
		}
		s.TreeRoots = treeRootsFromEntries(s.Entries)
		s.TreeExpanded = keep
		s.restoreTreeExpansions()
		s.rebuildTreeRows()
	}
	s.rebuildFilter()
	found := false
	if selectedName != "" {
		found = s.SelectVisibleEntry(selectedName)
	}
	if !found && indexFallback >= 0 && len(s.Entries) > 0 {
		if indexFallback >= len(s.Entries) {
			s.Cursor = len(s.Entries) - 1
		} else {
			s.Cursor = indexFallback
		}
	}
	if s.ListLayout == ListLayoutTree {
		switch {
		case sameDirReload && priorTreeCursorID != "":
			s.selectVisibleEntryByPath(priorTreeCursorID)
		case !sameDirReload && centerRecalled:
			if snap, ok := s.HistoryCursorByPath[cleanPathString(listingLoc.String())]; ok && snap.CursorPath != "" {
				s.selectVisibleEntryByPath(snap.CursorPath)
				s.treeCursorID = snap.CursorPath
			}
		}
	}
	if sameDirReload {
		s.finishSameDirectoryReloadScroll(priorCursor, priorScroll, viewportRows, wasCentered)
	} else {
		centerOnHighlight := s.ScrollMode == ScrollModeCenter || shouldCenterCursorOnListing(previousPath, listingLoc, centerRecalled, selectedName, indexFallback)
		s.applyHighlightScroll(viewportRows, centerOnHighlight)
	}
	if len(s.History) == 0 {
		cp := cleanPathString(listingLoc.String())
		if cp != "" {
			s.History = []string{cp}
			s.HistoryIndex = 0
		}
	}
	if s.OnDirectoryChange != nil {
		s.OnDirectoryChange()
	}
	return nil
}

func (s *State) prepareGitColumn(listingLoc pathloc.Path, entries []localfs.Entry) {
	// Any tree-child git-status fetches still in flight belong to the directory being left; their
	// eventual arrival will be rejected by applyGitStatusLoad's isWithinDir/GitColumnActive checks
	// anyway, so stop waiting on them rather than leaving the counter stuck non-zero forever.
	s.gitStatusChildPending = 0
	if listingLoc.IsRemote() {
		return
	}
	host, err := listingLoc.FilePath()
	if err != nil {
		s.GitColumnActive = false
		s.GitPending = false
		s.GitByPath = nil
		return
	}
	workRoot := gitignore.ValidWorkTreeRoot(host)
	s.GitColumnActive = workRoot != ""
	s.gitWorkRoot = workRoot
	s.GitByPath = nil
	if !s.GitColumnActive {
		s.GitPending = false
		return
	}
	if s.ScheduleGitStatus == nil {
		s.GitPending = false
		return
	}
	s.GitPending = true
	paths := make([]gitstatus.ListingPaths, len(entries))
	for i, e := range entries {
		paths[i] = gitstatus.ListingPaths{
			AbsPath: filepath.Clean(e.Path),
			IsDir:   e.Type == localfs.EntryDirectory,
		}
	}
	if !s.ScheduleGitStatus(GitStatusRequest{
		WorkRoot: workRoot,
		ListDir:  host,
		Paths:    paths,
	}) {
		s.GitPending = false
	}
}

// RescheduleGitStatusIfNeeded schedules an async git status fetch when the panel is inside a
// git work tree but no data has been loaded yet. Call this after wiring ScheduleGitStatus
// if the initial load ran before the scheduler was available.
func (s *State) RescheduleGitStatusIfNeeded() {
	if !s.GitColumnActive || s.GitPending || s.GitByPath != nil || s.ScheduleGitStatus == nil {
		return
	}
	if s.Path.IsRemote() {
		return
	}
	host, err := s.Path.FilePath()
	if err != nil {
		return
	}
	workRoot := gitignore.ValidWorkTreeRoot(host)
	if workRoot == "" {
		return
	}
	s.GitPending = true
	paths := make([]gitstatus.ListingPaths, len(s.Entries))
	for i, e := range s.Entries {
		paths[i] = gitstatus.ListingPaths{
			AbsPath: filepath.Clean(e.Path),
			IsDir:   e.Type == localfs.EntryDirectory,
		}
	}
	if !s.ScheduleGitStatus(GitStatusRequest{
		WorkRoot: workRoot,
		ListDir:  host,
		Paths:    paths,
	}) {
		s.GitPending = false
	}
}

func cleanPathString(p string) string {
	if p == "" {
		return ""
	}
	loc, err := pathloc.Parse(p)
	if err != nil {
		return ""
	}
	return loc.String()
}

func (s *State) refreshVolumeSpace(forPath pathloc.Path) {
	if forPath.IsRemote() {
		s.VolumeSpaceOK = false
		s.VolumeAvailBytes = 0
		s.VolumeTotalBytes = 0
		return
	}
	host, err := forPath.FilePath()
	if err != nil {
		s.VolumeSpaceOK = false
		return
	}
	avail, total, ok := fsvol.VolumeBytes(host)
	s.VolumeSpaceOK = ok
	if ok {
		s.VolumeAvailBytes = avail
		s.VolumeTotalBytes = total
		return
	}
	s.VolumeAvailBytes = 0
	s.VolumeTotalBytes = 0
}

// RefreshVolumeSpace re-samples free/total bytes for the volume containing Path without reloading the directory listing.
func (s *State) RefreshVolumeSpace() {
	s.refreshVolumeSpace(s.Path)
}

func (s *State) notifyChdir(oldPath, newPath pathloc.Path) {
	oldC := cleanPathString(oldPath.String())
	newC := cleanPathString(newPath.String())
	if oldC != newC {
		s.IdleDiskTotalsSort = false
	}
	if oldC == newC {
		return
	}
	if oldC != "" {
		s.appendLeftBehindSelectionsToStripOrder(oldC)
	}
	if newC != "" {
		s.stripSelectionsOrderForEnteredDir(newC)
	}
	s.normalizeSelectionsStripCursor()
}

func (s *State) appendLeftBehindSelectionsToStripOrder(leftDir string) {
	if len(s.SelectedPaths) == 0 {
		return
	}
	batch := make([]string, 0)
	for p := range s.SelectedPaths {
		if cleanPathString(filepath.Dir(p)) == leftDir {
			batch = append(batch, p)
		}
	}
	if len(batch) == 0 {
		return
	}
	sort.Strings(batch)
	for _, p := range batch {
		s.removePathFromSelectionsStripOrder(p)
		s.SelectionsStripOrder = append(s.SelectionsStripOrder, p)
	}
}

func (s *State) stripSelectionsOrderForEnteredDir(dir string) {
	if len(s.SelectionsStripOrder) == 0 {
		return
	}
	out := s.SelectionsStripOrder[:0]
	for _, p := range s.SelectionsStripOrder {
		if cleanPathString(filepath.Dir(p)) == dir {
			continue
		}
		out = append(out, p)
	}
	s.SelectionsStripOrder = out
}

func (s *State) removePathFromSelectionsStripOrder(path string) {
	if len(s.SelectionsStripOrder) == 0 {
		return
	}
	out := s.SelectionsStripOrder[:0]
	for _, p := range s.SelectionsStripOrder {
		if p != path {
			out = append(out, p)
		}
	}
	s.SelectionsStripOrder = out
}

// SelectionsStripPaths returns all selected paths shown in the selections sub-pane.
// Empty when every selection lives in the current directory (strip hidden).
func (s *State) SelectionsStripPaths() []string {
	s.ensureSelectionDerived()
	return s.selDerivedCache.stripPaths
}

// SelectionsStripCount is the number of rows in the selections sub-pane for the current directory.
func (s *State) SelectionsStripCount() int {
	s.ensureSelectionDerived()
	return len(s.selDerivedCache.stripPaths)
}

// MoveSelectionsStrip moves the selections-strip cursor by delta and keeps it visible.
func (s *State) MoveSelectionsStrip(delta int, stripViewportRows int) {
	n := s.SelectionsStripCount()
	if n == 0 {
		s.SelectionsStripCursor = 0
		s.SelectionsStripScroll = 0
		return
	}
	s.SelectionsStripCursor += delta
	s.normalizeSelectionsStripCursorWithViewport(stripViewportRows)
}

// SelectionsStripTop moves the selections-strip cursor to the first row.
func (s *State) SelectionsStripTop(stripViewportRows int) {
	if s.SelectionsStripCount() == 0 {
		return
	}
	s.SelectionsStripCursor = 0
	s.SelectionsStripScroll = 0
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

// SelectionsStripBottom moves the selections-strip cursor to the last row.
func (s *State) SelectionsStripBottom(stripViewportRows int) {
	n := s.SelectionsStripCount()
	if n == 0 {
		return
	}
	s.SelectionsStripCursor = n - 1
	s.EnsureSelectionsStripCursorVisible(stripViewportRows)
}

func (s *State) normalizeSelectionsStripCursor() {
	s.normalizeSelectionsStripCursorWithViewport(0)
}

// EnsureSelectionsStripCursorVisible clamps the selections-strip cursor and scroll to the viewport.
func (s *State) EnsureSelectionsStripCursorVisible(stripViewportRows int) {
	s.normalizeSelectionsStripCursorWithViewport(stripViewportRows)
}

func (s *State) normalizeSelectionsStripCursorWithViewport(stripViewportRows int) {
	n := s.SelectionsStripCount()
	if n == 0 {
		s.SelectionsStripCursor = 0
		s.SelectionsStripScroll = 0
		return
	}
	if s.SelectionsStripCursor < 0 {
		s.SelectionsStripCursor = 0
	}
	if s.SelectionsStripCursor >= n {
		s.SelectionsStripCursor = n - 1
	}
	if stripViewportRows <= 0 {
		if s.SelectionsStripScroll > s.SelectionsStripCursor {
			s.SelectionsStripScroll = s.SelectionsStripCursor
		}
		return
	}
	maxOffset := n - stripViewportRows
	if maxOffset < 0 {
		maxOffset = 0
		s.SelectionsStripScroll = 0
	}
	if s.SelectionsStripScroll > maxOffset {
		s.SelectionsStripScroll = maxOffset
	}
	if s.SelectionsStripScroll < 0 {
		s.SelectionsStripScroll = 0
	}
	if s.SelectionsStripCursor < s.SelectionsStripScroll {
		s.SelectionsStripScroll = s.SelectionsStripCursor
	}
	if s.SelectionsStripCursor >= s.SelectionsStripScroll+stripViewportRows {
		s.SelectionsStripScroll = s.SelectionsStripCursor - stripViewportRows + 1
	}
}

// SelectedPathAtStripIndex returns the absolute path at the given strip row index, or "", false.
func (s *State) SelectedPathAtStripIndex(index int) (string, bool) {
	paths := s.SelectionsStripPaths()
	if index < 0 || index >= len(paths) {
		return "", false
	}
	return paths[index], true
}

// ToggleOrRemoveStripSelection toggles off the strip row at SelectionsStripCursor (deselects).
func (s *State) ToggleOrRemoveStripSelection() bool {
	p, ok := s.SelectedPathAtStripIndex(s.SelectionsStripCursor)
	if !ok {
		return false
	}
	wasDir := s.selectedPathIsDirectory(p)
	s.applySelectionRemove(p, wasDir)
	s.removePathFromSelectionsStripOrder(p)
	s.normalizeSelectionsStripCursor()
	return true
}

func (s *State) SelectVisibleEntry(name string) bool {
	for i := 0; i < s.VisibleEntryCount(); i++ {
		entry, _, ok := s.VisibleEntry(i)
		if !ok {
			continue
		}
		if entry.Name == name {
			s.Cursor = i
			return true
		}
	}
	return false
}

// SelectVisibleEntryInViewport selects by visible name and scrolls according to ScrollMode.
func (s *State) SelectVisibleEntryInViewport(name string, viewportRows int) bool {
	if !s.SelectVisibleEntry(name) {
		return false
	}
	s.EnsureCursorInViewport(viewportRows)
	return true
}

// SelectVisibleEntryCentered selects by visible name and scrolls to center the row when possible.
// Used after in-place operations (rename) so scroll matches explicit navigation recall, independent of ScrollMode.
func (s *State) SelectVisibleEntryCentered(name string, viewportRows int) bool {
	if !s.SelectVisibleEntry(name) {
		return false
	}
	s.applyHighlightScroll(viewportRows, true)
	return true
}

func (s *State) selectVisibleEntryByPath(absPath string) {
	if absPath == "" {
		return
	}
	wantLoc, wantErr := pathloc.Parse(absPath)
	for i := 0; i < s.VisibleEntryCount(); i++ {
		entry, _, ok := s.VisibleEntry(i)
		if !ok {
			continue
		}
		if wantErr == nil {
			if entLoc, err := pathloc.Parse(entry.Path); err == nil && entLoc.Equal(wantLoc) {
				s.Cursor = i
				return
			}
		}
		if filepath.Clean(entry.Path) == filepath.Clean(absPath) {
			s.Cursor = i
			return
		}
	}
}

func (s *State) clampCursor() {
	count := s.VisibleEntryCount()
	if count == 0 {
		s.Cursor = 0
		s.ScrollOffset = 0
		return
	}
	if s.Cursor < 0 {
		s.Cursor = 0
	}
	if s.Cursor >= count {
		s.Cursor = count - 1
	}
}

func (s *State) applyFilterQuery(query string, viewportRows int) {
	s.Filter.Query = query
	s.Filter.Cursor = lineedit.ClampRuneCursor(s.Filter.Cursor, len([]rune(query)))
	s.Filter.Active = query != ""
	s.rebuildFilter()
	if len(s.Filter.results) > 0 {
		s.Cursor = primaryFilterMatchIndex(query, s.Filter.results)
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// primaryFilterMatchIndex picks the cursor row after the query changes.
// A single typed letter (after trim) uses the first visible match; longer queries use the best ranked match.
func primaryFilterMatchIndex(query string, ranked []filterResult) int {
	if len(ranked) == 0 {
		return 0
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return ranked[0].Index
	}
	if len([]rune(q)) == 1 {
		best := ranked[0].Index
		for _, r := range ranked[1:] {
			if r.Index < best {
				best = r.Index
			}
		}
		return best
	}
	return ranked[0].Index
}

func (s *State) rebuildFilter() {
	s.rebuildEntryFilter()
	s.Filter.results = nil
	if s.Filter.Query == "" {
		s.Filter.Active = false
		return
	}

	query := search.Parse(s.Filter.Query)
	if query.Empty() {
		s.Filter.Active = false
		return
	}

	count := s.VisibleEntryCount()
	names := make([]string, count)
	for i := 0; i < count; i++ {
		entry, _, _ := s.VisibleEntry(i)
		names[i] = entry.Name
	}
	ranked := query.Rank(names, search.Options{CaseInsensitive: s.Filter.CaseInsensitive})
	s.Filter.results = make([]filterResult, 0, len(ranked))
	for _, result := range ranked {
		s.Filter.results = append(s.Filter.results, filterResult{
			Index:  result.Index,
			Score:  result.Result.Score,
			Ranges: result.Result.Ranges,
		})
	}
	s.Filter.Active = true
}

// InvertSelection toggles selection for all visible entries.
func (s *State) InvertSelection() {
	count := s.VisibleEntryCount()
	if count == 0 {
		return
	}
	listingSet := make(map[string]bool, count)
	newSel := make(map[string]bool, count)
	hasDirs := false
	for i := 0; i < count; i++ {
		entry, _, ok := s.VisibleEntry(i)
		if !ok {
			continue
		}
		listingSet[entry.Path] = true
		wasSelected := s.SelectedPaths != nil && s.SelectedPaths[entry.Path]
		if wasSelected {
			continue
		}
		newSel[entry.Path] = true
		if entry.IsDir() {
			hasDirs = true
		}
	}
	// Keep off-listing selections that were not inverted.
	if s.SelectedPaths != nil {
		for p, on := range s.SelectedPaths {
			if !on || listingSet[p] {
				continue
			}
			newSel[p] = true
			if s.SelectedDirPaths != nil && s.SelectedDirPaths[p] {
				hasDirs = true
			} else if s.selectedPathIsDirectory(p) {
				hasDirs = true
			}
		}
	}
	if len(newSel) == 0 {
		s.clearSelectionState()
		s.SelectionsStripOrder = rebuildSelectionsStripOrderAfterBulk(s.SelectionsStripOrder, nil)
		s.invalidateSelectionDerivedFull()
	} else {
		s.SelectedPaths = newSel
		s.selectionHasDirs = hasDirs
		if hasDirs {
			s.rebuildSelectedDirPaths()
		} else {
			s.SelectedDirPaths = nil
		}
		s.recomputeSelectionListedBytes()
		s.SelectionsStripOrder = rebuildSelectionsStripOrderAfterBulk(s.SelectionsStripOrder, newSel)
		s.invalidateSelectionDerivedFull()
	}
	s.normalizeSelectionsStripCursor()
}

func rebuildSelectionsStripOrderAfterBulk(stripOrder []string, selected map[string]bool) []string {
	if len(stripOrder) == 0 {
		return nil
	}
	if selected == nil {
		return nil
	}
	out := make([]string, 0, len(stripOrder))
	for _, p := range stripOrder {
		if selected[p] {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ClearSelection removes all tagged paths on this panel (any directory) and clears the
// selections strip order.
func (s *State) ClearSelection() {
	s.clearSelectionState()
	s.SelectionsStripOrder = nil
	s.invalidateSelectionDerivedFull()
	s.normalizeSelectionsStripCursor()
}

// SelectGroup selects entries whose basename (or any meta column value) matches the pattern.
// filesOnly: when true, only regular files are matched (directories skipped).
// dirsOnly: when true, only directories are matched (regular files skipped).
// caseSensitive: when true, matching is case-sensitive (shell and simple modes only).
// meta: optional meta column data; when Cols is non-empty, values are also matched; OnlyMeta skips filename.
// The returned bool reports whether the pattern matched any entry.
func (s *State) SelectGroup(pattern string, filesOnly, dirsOnly, caseSensitive bool, mode GroupPatternMode, meta GroupSelectMeta) (bool, error) {
	paths, isDir, err := s.groupMatchedPaths(pattern, filesOnly, dirsOnly, caseSensitive, mode, meta)
	if err != nil {
		return false, err
	}
	if len(paths) == 0 {
		return false, nil
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool, len(paths))
	}
	lookup := func(path string) bool {
		return isDir[path]
	}
	_ = BulkApplySelectionAdds(s.SelectedPaths, paths, lookup)
	s.rebuildSelectedDirPaths()
	s.recomputeSelectionListedBytes()
	s.invalidateSelectionDerivedFull()
	return true, nil
}

// groupMatchedPaths returns the visible entries (paths and dir flags) matching pattern under
// the given filters. Shared matching pipeline for SelectGroup, UnselectGroup, and CountGroupMatches.
func (s *State) groupMatchedPaths(pattern string, filesOnly, dirsOnly, caseSensitive bool, mode GroupPatternMode, meta GroupSelectMeta) ([]string, map[string]bool, error) {
	matcher, err := NewGroupMatcher(pattern, mode, caseSensitive)
	if err != nil {
		return nil, nil, err
	}
	paths := make([]string, 0)
	isDir := make(map[string]bool)
	for i := 0; i < s.VisibleEntryCount(); i++ {
		entry, _, ok := s.VisibleEntry(i)
		if !ok {
			continue
		}
		if filesOnly && entry.IsDir() {
			continue
		}
		if dirsOnly && !entry.IsDir() {
			continue
		}
		matched := !meta.OnlyMeta && matcher.Match(entry.Name)
		if !matched {
			for _, col := range meta.Cols {
				if v, ok := col[entry.Path]; ok && v != "" && matcher.Match(v) {
					matched = true
					break
				}
			}
		}
		if matched {
			paths = append(paths, entry.Path)
			isDir[entry.Path] = entry.IsDir()
		}
	}
	return paths, isDir, nil
}

// CountGroupMatches reports how many visible entries matching pattern have selection state
// equal to selected — i.e. how many entries SelectGroup (selected=false) or UnselectGroup
// (selected=true) would actually change — split into files and directories, for the
// group-select dialog's live result preview.
func (s *State) CountGroupMatches(pattern string, filesOnly, dirsOnly, caseSensitive bool, mode GroupPatternMode, meta GroupSelectMeta, selected bool) (files, dirs int, err error) {
	paths, isDir, err := s.groupMatchedPaths(pattern, filesOnly, dirsOnly, caseSensitive, mode, meta)
	if err != nil {
		return 0, 0, err
	}
	for _, p := range paths {
		if s.SelectedPaths[p] != selected {
			continue
		}
		if isDir[p] {
			dirs++
		} else {
			files++
		}
	}
	return files, dirs, nil
}

// UnselectGroup unselects entries whose basename (or any meta column value) matches the pattern.
// meta: optional meta column data; when Cols is non-empty, values are also matched; OnlyMeta skips filename.
// The returned bool reports whether the pattern matched any entry (regardless of prior selection state).
func (s *State) UnselectGroup(pattern string, filesOnly, dirsOnly, caseSensitive bool, mode GroupPatternMode, meta GroupSelectMeta) (bool, error) {
	paths, isDir, err := s.groupMatchedPaths(pattern, filesOnly, dirsOnly, caseSensitive, mode, meta)
	if err != nil {
		return false, err
	}
	if len(paths) == 0 {
		return false, nil
	}
	for _, path := range paths {
		if !s.SelectedPaths[path] {
			continue
		}
		s.applySelectionRemove(path, isDir[path])
		s.removePathFromSelectionsStripOrder(path)
	}
	s.normalizeSelectionsStripCursor()
	return true, nil
}

// RefreshDiskUsageOrdering reapplies cached disk-total ordering when subtree sizes update while staying in one directory.
// When centerCursor is true (typically the first application after idle timeout), the highlighted row is kept by path and scroll is centered on it.
func (s *State) RefreshDiskUsageOrdering(viewportRows int, centerCursor bool) {
	if len(s.Entries) == 0 || !s.Sort.DiskUsageIdleSizeSort || !s.IdleDiskTotalsSort {
		return
	}
	entry, ok := s.CurrentEntry()
	selectedPath := ""
	if ok {
		selectedPath = filepath.Clean(entry.Path)
	}
	liveVR := s.effectiveFileListViewportRows(viewportRows)
	wasCentered := s.cursorAppearsCentered(liveVR)
	wasEdged := s.cursorAppearsEdged(liveVR)
	s.ApplySort()
	s.rebuildFilter()
	if selectedPath != "" {
		s.selectVisibleEntryByPath(selectedPath)
	}
	switch {
	case centerCursor || s.ScrollMode == ScrollModeCenter || wasCentered:
		s.applyHighlightScroll(viewportRows, true)
	case s.ScrollMode == ScrollModeEdge && wasEdged:
		s.EnsureCursorEdge(liveVR)
	default:
		s.applyHighlightScroll(viewportRows, false)
	}
}

// ApplySortFromDialog assigns panel SortState from the modal while preserving idle-disk-sort flags sensibly.
func (s *State) ApplySortFromDialog(sort SortState, viewportRows int) {
	prev := s.Sort
	s.Sort = sort
	if prev.Mode != sort.Mode || prev.Reverse != sort.Reverse || prev.DirectoriesFirst != sort.DirectoriesFirst {
		s.IdleDiskTotalsSort = false
	}
	if !sort.DiskUsageIdleSizeSort {
		s.IdleDiskTotalsSort = false
		s.DiskUsageIdleSortActivated = false
	} else {
		s.DiskUsageIdleSortActivated = true
	}
	entry, ok := s.CurrentEntry()
	selectedName := ""
	if ok {
		selectedName = entry.Name
	}
	s.ApplySort()
	s.rebuildFilter()
	if selectedName != "" {
		_ = s.SelectVisibleEntry(selectedName)
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// ListingFullyDiskCached reports whether every listed entry has a disk-total/file aggregate in DiskSorter.
func (s *State) ListingFullyDiskCached() bool {
	if s.DiskSorter == nil || len(s.Entries) == 0 {
		return false
	}
	for _, e := range s.Entries {
		if _, ok := s.DiskSorter(filepath.Clean(e.Path)); !ok {
			return false
		}
	}
	return true
}

// SetSortMode changes the sort mode, applies it, and preserves the cursor by path.
func (s *State) SetSortMode(mode SortMode, reverse bool, dirsFirst bool, viewportRows int) {
	s.IdleDiskTotalsSort = false
	entry, ok := s.CurrentEntry()
	selectedName := ""
	if ok {
		selectedName = entry.Name
	}
	s.Sort.Mode = mode
	s.Sort.Reverse = reverse
	s.Sort.DirectoriesFirst = dirsFirst
	s.ApplySort()
	s.rebuildFilter()
	if selectedName != "" {
		_ = s.SelectVisibleEntry(selectedName)
	}
	s.clampCursor()
	s.EnsureCursorInViewport(viewportRows)
}

// CycleSort cycles through sort modes (name → extension → size → mtime → name).
func (s *State) CycleSort(viewportRows int) {
	modes := IterateSortModes()
	next := 0
	for i, m := range modes {
		if m == s.Sort.Mode {
			next = (i + 1) % len(modes)
			break
		}
	}
	s.SetSortMode(modes[next], s.Sort.Reverse, s.Sort.DirectoriesFirst, viewportRows)
}

// CycleListingFormat cycles listing format: mtime → perm → brief → mtime.
func (s *State) CycleListingFormat() {
	formats := IterateListFormats()
	cur := EffectiveListFormat(s.ListFormat)
	next := 0
	for i, f := range formats {
		if f == cur {
			next = (i + 1) % len(formats)
			break
		}
	}
	s.ListFormat = formats[next]
}

// ToggleSortReverse flips the reverse flag and re-sorts.
func (s *State) ToggleSortReverse(viewportRows int) {
	s.SetSortMode(s.Sort.Mode, !s.Sort.Reverse, s.Sort.DirectoriesFirst, viewportRows)
}

func (s State) currentEntryName() string {
	entry, ok := s.CurrentEntry()
	if !ok {
		return ""
	}
	return entry.Name
}
