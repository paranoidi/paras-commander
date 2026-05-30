package panel

import (
	"context"
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/fsbackend/file"
	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/search"
)

const maxNavHistory = 200

// noIndexCursorFallback is passed to load as indexFallback to keep cursor at 0 when the
// preserve-by-name step cannot resolve (directory changes, initial load).
const noIndexCursorFallback = -1

// historyCursorSnapshot stores the highlighted row when leaving a directory so re-entry can restore it.
type historyCursorSnapshot struct {
	EntryName string
	Index     int
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
	// GitColumnActive is true when the listing path is inside a Git work tree (local panels only).
	GitColumnActive bool
	// GitPending is true while async git status is in flight for this listing.
	GitPending bool
	// GitByPath maps absolute entry paths to eza-style staged/unstaged cells; nil until loaded.
	GitByPath map[string]gitstatus.Cell
	Filter    FilterState
	// DiskSorter returns cached subtree or file aggregates for Disk usage sorting; absent cache ranks last until known.
	DiskSorter func(absPath string) (int64, bool)
	// InDiskUsageScanScope, when set by the app, reports whether this panel cwd is within the active disk scan.
	InDiskUsageScanScope func() bool
	Sort                 SortState
	// ListFormat controls trailing columns after size (Modified / Permissions / none). Per-panel; see config default_listing_format.
	ListFormat ListFormat
	// CenterScrolling mirrors [ui].center_scrolling: navigation keeps the highlight row centered when true.
	CenterScrolling bool
	// CarouselMode shows a three-column parent | current | child preview inside this panel.
	CarouselMode bool
	// CarouselChildPreviewCoalesce skips child-directory reads during scroll and reuses CarouselSideCache.Child.
	CarouselChildPreviewCoalesce bool
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
	// IdleDiskTotalsSort is set after disk scan completes and idle-sort delay elapses (DiskUsageIdleSizeSort).
	IdleDiskTotalsSort bool
	// DiskUsageIdleSortActivated mirrors the disk-usage sort checkbox lifecycle (config/dialog apply).
	// Idle-sort scheduling keys off Sort.DiskUsageIdleSizeSort; this flag stays in sync for UI/state parity.
	DiskUsageIdleSortActivated bool

	// NewFileMarksByDir maps listing directory paths to entry base names marked "new"
	// after a completed copy/move/flatten into that directory. Marks are dropped when leaving the directory.
	NewFileMarksByDir map[string]map[string]struct{}

	// OnDirectoryChange is called after every successful directory load (Enter, Parent,
	// HistoryBackward/Forward, Refresh, ToggleHidden, etc.). The app uses this to check whether disk-usage idle sorting
	// can be applied immediately or needs to be deferred.
	OnDirectoryChange func()
	// SuppressHeavyPathProbes, when set, skips statfs and device lookup in load() for paths
	// where those syscalls would contend with active background job I/O on the same volume.
	SuppressHeavyPathProbes func(pathloc.Path) bool
	// ScheduleRemoteLoad runs remote directory listing off the UI thread (set by app; nil = synchronous).
	ScheduleRemoteLoad RemoteLoadScheduler
	// ListingPending is true while an asynchronous remote listing is in flight.
	ListingPending bool
	// ScheduleGitStatus runs git status for the current listing off the UI thread (set by app).
	ScheduleGitStatus GitStatusScheduler
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
	return s.loadPathString(path, "", 0, noIndexCursorFallback, remoteLoadOpts{})
}

// Refresh reloads the current path. When the entry under the cursor still exists, it is re-selected by name;
// otherwise the prior row index is restored (clamped), matching MC-style behavior after moves/deletes.
func (s *State) Refresh(viewportRows int) error {
	priorCursor := s.Cursor
	entry, ok := s.CurrentEntry()
	selectedName := ""
	if ok {
		selectedName = entry.Name
	}
	return s.load(s.Path, selectedName, viewportRows, priorCursor, remoteLoadOpts{})
}

// ToggleHidden flips hidden-file visibility and reloads the current directory using the same cursor rules as Refresh.
func (s *State) ToggleHidden(viewportRows int) error {
	priorCursor := s.Cursor
	entry, ok := s.CurrentEntry()
	selectedName := ""
	if ok {
		selectedName = entry.Name
	}
	s.ShowHidden = !s.ShowHidden
	return s.load(s.Path, selectedName, viewportRows, priorCursor, remoteLoadOpts{})
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
	return len(s.Entries)
}

// VisibleEntry returns the visible entry and its backing Entries index.
func (s State) VisibleEntry(index int) (localfs.Entry, int, bool) {
	if index < 0 || index >= s.VisibleEntryCount() {
		return localfs.Entry{}, 0, false
	}
	entryIndex := index
	if entryIndex < 0 || entryIndex >= len(s.Entries) {
		return localfs.Entry{}, 0, false
	}
	return s.Entries[entryIndex], entryIndex, true
}

// FilterHasMatches reports whether the active quick filter has at least one file-name match.
func (s State) FilterHasMatches() bool {
	return len(s.Filter.results) > 0
}

// MatchRanges returns highlighted rune ranges for the visible entry.
func (s State) MatchRanges(index int) []search.Range {
	if !s.Filter.Active || index < 0 || index >= len(s.Entries) {
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
func (s *State) AddSelection(path string) {
	path = cleanPathString(path)
	if path == "" {
		return
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	if s.SelectedPaths[path] {
		return
	}
	s.SelectedPaths[path] = true
}

// ToggleSelection toggles the current entry in the panel-local selection set.
func (s *State) ToggleSelection() bool {
	entry, ok := s.CurrentEntry()
	if !ok {
		return false
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	if s.SelectedPaths[entry.Path] {
		delete(s.SelectedPaths, entry.Path)
		s.removePathFromSelectionsStripOrder(entry.Path)
		s.normalizeSelectionsStripCursor()
		return false
	}
	s.SelectedPaths[entry.Path] = true
	return true
}

// ToggleSelectionAndAdvance toggles the current entry, then moves to the next row when possible.
func (s *State) ToggleSelectionAndAdvance(viewportRows int) bool {
	if _, ok := s.CurrentEntry(); !ok {
		return false
	}
	s.ToggleSelection()
	if s.Cursor < s.VisibleEntryCount()-1 {
		s.Move(1, viewportRows)
	}
	return true
}

// IsSelected reports whether entry is selected in this panel.
func (s State) IsSelected(entry localfs.Entry) bool {
	return s.SelectedPaths != nil && s.SelectedPaths[entry.Path]
}

// HasSelectionInSubtree reports whether some selected path is a strict descendant of dirPath.
func (s State) HasSelectionInSubtree(dirPath string) bool {
	if s.SelectedPaths == nil {
		return false
	}
	dir := cleanPathString(dirPath)
	if dir == "" {
		return false
	}
	for p, on := range s.SelectedPaths {
		if !on {
			continue
		}
		if isStrictPathDescendant(dir, cleanPathString(p)) {
			return true
		}
	}
	return false
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
	if len(s.SelectedPaths) == 0 {
		return nil
	}
	byPath := make(map[string]localfs.Entry, len(s.Entries))
	for _, entry := range s.Entries {
		byPath[entry.Path] = entry
	}
	out := make([]string, 0)
	for path, on := range s.SelectedPaths {
		if !on {
			continue
		}
		path = cleanPathString(path)
		if path == "" {
			continue
		}
		if e, ok := byPath[path]; ok {
			if e.Type == localfs.EntryDirectory {
				out = append(out, path)
			}
			continue
		}
		e, err := localfs.EntryFromPath(path)
		if err != nil {
			continue
		}
		if e.Type == localfs.EntryDirectory {
			out = append(out, path)
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Strings(out)
	return out
}

// PruneNestedPaths drops paths that are strict descendants of another path in the slice.
func PruneNestedPaths(paths []string) []string {
	if len(paths) <= 1 {
		if len(paths) == 1 {
			return []string{cleanPathString(paths[0])}
		}
		return nil
	}
	sorted := make([]string, len(paths))
	for i, p := range paths {
		sorted[i] = cleanPathString(p)
	}
	sort.Strings(sorted)
	out := make([]string, 0, len(sorted))
	for _, p := range sorted {
		if p == "" {
			continue
		}
		nested := false
		for _, kept := range out {
			if isStrictPathDescendant(kept, p) {
				nested = true
				break
			}
		}
		if nested {
			continue
		}
		trimmed := out[:0]
		for _, kept := range out {
			if !isStrictPathDescendant(p, kept) {
				trimmed = append(trimmed, kept)
			}
		}
		out = append(trimmed, p)
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
	if err := s.load(loc, selectedName, viewportRows, noIndexCursorFallback, remoteLoadOpts{
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
	if err := s.loadPathString(target, "", viewportRows, noIndexCursorFallback, remoteLoadOpts{
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
	if err := s.loadPathString(target, "", viewportRows, noIndexCursorFallback, remoteLoadOpts{
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
	if entry, ok := s.CurrentEntry(); ok {
		name = entry.Name
	}
	if s.HistoryCursorByPath == nil {
		s.HistoryCursorByPath = make(map[string]historyCursorSnapshot)
	}
	s.HistoryCursorByPath[dir] = historyCursorSnapshot{EntryName: name, Index: s.Cursor}
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
// same-directory reloads (Refresh, ToggleHidden) keep minimal scroll.
func shouldCenterCursorOnListing(previousPath, listingLoc pathloc.Path, centerRecalled bool, selectedName string, indexFallback int) bool {
	if previousPath.Equal(listingLoc) {
		return false
	}
	return centerRecalled || selectedName != "" || indexFallback >= 0
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

// EnsureCursorInViewport scrolls so the cursor stays visible, or centered when CenterScrolling is set.
func (s *State) EnsureCursorInViewport(viewportRows int) {
	if s.CenterScrolling {
		s.EnsureCursorCentered(viewportRows)
		return
	}
	s.EnsureCursorVisible(viewportRows)
}

// SetFilterCaseInsensitive configures the quick filter case behavior.
func (s *State) SetFilterCaseInsensitive(value bool, viewportRows int) {
	s.Filter.CaseInsensitive = value
	selectedName := s.currentEntryName()
	s.rebuildFilter()
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

// AppendFilterRune appends a printable rune to the query.
func (s *State) AppendFilterRune(value rune, viewportRows int) {
	s.Filter.Editing = true
	s.applyFilterQuery(s.Filter.Query+string(value), viewportRows)
}

// BackspaceFilter removes the last rune from the query.
func (s *State) BackspaceFilter(viewportRows int) {
	runes := []rune(s.Filter.Query)
	if len(runes) == 0 {
		s.Filter.Editing = false
		return
	}
	s.Filter.Editing = true
	s.applyFilterQuery(string(runes[:len(runes)-1]), viewportRows)
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

func (s *State) loadPathString(path string, selectedName string, viewportRows int, indexFallback int, remote remoteLoadOpts) error {
	loc, err := pathloc.Parse(path)
	if err != nil {
		return err
	}
	return s.load(loc, selectedName, viewportRows, indexFallback, remote)
}

func (s *State) load(loc pathloc.Path, selectedName string, viewportRows int, indexFallback int, remote remoteLoadOpts) error {
	if !loc.Equal(s.Path) {
		s.rememberCursorForPath(s.Path.String())
		s.dropNewFileMarks(s.Path.String())
	}
	selectedName, indexFallback, centerRecalled := s.resolveLoadCursor(loc, selectedName, indexFallback)
	if loc.IsRemote() && s.ScheduleRemoteLoad != nil {
		if s.ScheduleRemoteLoad(RemoteLoadRequest{
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
	if loc.IsRemote() {
		be, berr := fsbackend.Default().Backend(loc)
		if berr != nil {
			return nil, pathloc.Path{}, false, false, berr
		}
		entries, err := be.List(context.Background(), loc)
		if err != nil {
			return nil, pathloc.Path{}, false, false, err
		}
		dotfilesHiddenActive := !s.ShowHidden && fsbackend.HasDotfileNames(entries)
		return fsbackend.FilterHidden(entries, s.ShowHidden), loc, false, dotfilesHiddenActive, nil
	}
	host, ferr := loc.FilePath()
	if ferr != nil {
		return nil, pathloc.Path{}, false, false, ferr
	}
	gitMatcher, gerr := localfs.MatcherForListing(s.ShowHidden, s.Gitignore, host)
	if gerr != nil {
		return nil, pathloc.Path{}, false, false, gerr
	}
	be := file.New()
	entries, err := be.ListWithOptions(context.Background(), loc, localfs.ListOptions{
		ShowHidden: s.ShowHidden,
		Gitignore:  gitMatcher,
	})
	if err != nil {
		return nil, pathloc.Path{}, false, false, err
	}
	dotfilesHiddenActive := false
	if !s.ShowHidden {
		dotfilesHiddenActive, err = localfs.DirHasDotfileNames(host)
		if err != nil {
			return nil, pathloc.Path{}, false, false, err
		}
	}
	listingLoc, err := pathloc.File(host)
	if err != nil {
		return nil, pathloc.Path{}, false, false, err
	}
	return entries, listingLoc, gitMatcher != nil, dotfilesHiddenActive, nil
}

// ApplyListing commits backend entries into panel state (used after sync or async remote list).
func (s *State) ApplyListing(listingLoc pathloc.Path, backendEntries []fsbackend.Entry, selectedName string, viewportRows int, indexFallback int, centerRecalled bool) error {
	localEntries, err := fsbackend.ToPanelEntries(backendEntries)
	if err != nil {
		return err
	}

	previousPath := s.Path
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
	s.Entries = localEntries
	s.Cursor = 0
	s.ScrollOffset = 0
	if !previousPath.Equal(listingLoc) {
		s.invalidateCarouselChildCache()
		s.notifyChdir(previousPath, listingLoc)
	}
	// Activate disk-total primary sort before ApplySort so the first paint matches MC-style
	// disk ordering when cache already covers this listing (no idle timer / reconcile delay).
	if s.Sort.DiskUsageIdleSizeSort && len(s.Entries) > 0 && s.ListingFullyDiskCached() && s.inDiskUsageScanScope() {
		s.DiskUsageIdleSortActivated = true
		s.IdleDiskTotalsSort = true
	}
	s.ApplySort()
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
	s.clampCursor()
	if s.CenterScrolling {
		s.EnsureCursorCentered(viewportRows)
	} else if shouldCenterCursorOnListing(previousPath, listingLoc, centerRecalled, selectedName, indexFallback) {
		s.EnsureCursorCentered(viewportRows)
	} else {
		s.EnsureCursorVisible(viewportRows)
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
	workRoot := gitignore.WorkTreeRoot(host)
	s.GitColumnActive = workRoot != ""
	if !s.GitColumnActive {
		s.GitPending = false
		s.GitByPath = nil
		return
	}
	s.GitPending = true
	s.GitByPath = nil
	if s.ScheduleGitStatus == nil {
		return
	}
	paths := make([]gitstatus.ListingPaths, len(entries))
	for i, e := range entries {
		paths[i] = gitstatus.ListingPaths{
			AbsPath: filepath.Clean(e.Path),
			IsDir:   e.Type == localfs.EntryDirectory,
		}
	}
	s.ScheduleGitStatus(GitStatusRequest{
		WorkRoot: workRoot,
		ListDir:  host,
		Paths:    paths,
	})
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

// SelectionsStripPaths returns selected paths shown in the selections sub-pane (not in the current directory).
func (s *State) SelectionsStripPaths() []string {
	cur := cleanPathString(s.Path.String())
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

// SelectionsStripCount is the number of rows in the selections sub-pane for the current directory.
func (s *State) SelectionsStripCount() int {
	return len(s.SelectionsStripPaths())
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
	if s.SelectedPaths != nil {
		delete(s.SelectedPaths, p)
		if len(s.SelectedPaths) == 0 {
			s.SelectedPaths = nil
		}
	}
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

	names := make([]string, len(s.Entries))
	for i, entry := range s.Entries {
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
	if len(s.Entries) == 0 {
		return
	}
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	for _, entry := range s.Entries {
		if s.SelectedPaths[entry.Path] {
			delete(s.SelectedPaths, entry.Path)
			s.removePathFromSelectionsStripOrder(entry.Path)
		} else {
			s.SelectedPaths[entry.Path] = true
		}
	}
	if len(s.SelectedPaths) == 0 {
		s.SelectedPaths = nil
	}
	s.normalizeSelectionsStripCursor()
}

// ClearSelection removes all tagged paths on this panel (any directory) and clears the
// selections strip order.
func (s *State) ClearSelection() {
	s.SelectedPaths = nil
	s.SelectionsStripOrder = nil
	s.normalizeSelectionsStripCursor()
}

// SelectGroup selects entries whose basename matches the pattern.
// filesOnly: when true, only regular files are matched (directories skipped).
// caseSensitive: when true, matching is case-sensitive.
// useShellPatterns: when true, uses filepath.Match; otherwise substring match.
func (s *State) SelectGroup(pattern string, filesOnly, caseSensitive, useShellPatterns bool) {
	if s.SelectedPaths == nil {
		s.SelectedPaths = make(map[string]bool)
	}
	for _, entry := range s.Entries {
		if filesOnly && entry.IsDir() {
			continue
		}
		if groupMatch(entry.Name, pattern, caseSensitive, useShellPatterns) {
			s.SelectedPaths[entry.Path] = true
		}
	}
	if len(s.SelectedPaths) == 0 {
		s.SelectedPaths = nil
	}
}

// UnselectGroup unselects entries whose basename matches the pattern.
func (s *State) UnselectGroup(pattern string, filesOnly, caseSensitive, useShellPatterns bool) {
	for _, entry := range s.Entries {
		if filesOnly && entry.IsDir() {
			continue
		}
		if groupMatch(entry.Name, pattern, caseSensitive, useShellPatterns) {
			delete(s.SelectedPaths, entry.Path)
			s.removePathFromSelectionsStripOrder(entry.Path)
		}
	}
	if len(s.SelectedPaths) == 0 {
		s.SelectedPaths = nil
	}
	s.normalizeSelectionsStripCursor()
}

// groupMatch returns true if name matches pattern according to the given options.
func groupMatch(name, pattern string, caseSensitive, useShellPatterns bool) bool {
	if useShellPatterns {
		if caseSensitive {
			matched, _ := filepath.Match(pattern, name)
			return matched
		}
		matched, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(name))
		return matched
	}
	// Simple substring match
	n := name
	p := pattern
	if !caseSensitive {
		n = strings.ToLower(n)
		p = strings.ToLower(p)
	}
	return strings.Contains(n, p)
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
	s.ApplySort()
	s.rebuildFilter()
	if selectedPath != "" {
		s.selectVisibleEntryByPath(selectedPath)
	}
	s.clampCursor()
	if centerCursor || s.CenterScrolling {
		s.EnsureCursorCentered(viewportRows)
	} else {
		s.EnsureCursorVisible(viewportRows)
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
