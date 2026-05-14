package panel

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/search"
)

const maxNavHistory = 200

// noIndexCursorFallback is passed to load as indexFallback to keep cursor at 0 when the
// preserve-by-name step cannot resolve (directory changes, initial load, history navigation).
const noIndexCursorFallback = -1

// State contains all panel data needed by the App and renderer.
type State struct {
	Path string
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
	History       []string
	HistoryIndex  int
	SelectedPaths map[string]bool
	// SelectionsStripOrder lists selected paths that belong in the bottom “selections” strip:
	// off-current-directory order is tracked here; on chdir into dir D those paths are removed;
	// on chdir away from D, selected paths under D are appended (deduped).
	SelectionsStripOrder  []string
	SelectionsStripCursor int
	SelectionsStripScroll int
	ShowHidden            bool
	Filter                FilterState
	// DiskSorter returns cached subtree or file aggregates for Disk usage sorting; absent cache ranks last until known.
	DiskSorter func(absPath string) (int64, bool)
	Sort       SortState
	// ListFormat controls trailing columns after size (Modified / Permissions / none). Per-panel; see config default_listing_format.
	ListFormat ListFormat
	// IdleDiskTotalsSort is set after disk scan completes and idle-sort delay elapses (DiskUsageIdleSizeSort).
	IdleDiskTotalsSort bool
	// DiskUsageIdleSortActivated mirrors the disk-usage sort checkbox lifecycle (config/dialog apply).
	// Idle-sort scheduling keys off Sort.DiskUsageIdleSizeSort; this flag stays in sync for UI/state parity.
	DiskUsageIdleSortActivated bool

	// OnDirectoryChange is called after every successful directory load (Enter, Parent,
	// HistoryBackward/Forward, Refresh, ToggleHidden, etc.). The app uses this to check whether disk-usage idle sorting
	// can be applied immediately or needs to be deferred.
	OnDirectoryChange func()
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
	return NewWithOptions(path, localfs.DefaultListOptions())
}

// NewWithOptions loads a panel rooted at path with configured listing defaults.
func NewWithOptions(path string, opts localfs.ListOptions) (State, error) {
	state := State{
		Cursor:       0,
		ScrollOffset: 0,
		ShowHidden:   opts.ShowHidden,
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
	return s.load(path, "", 0, noIndexCursorFallback)
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
	return s.load(s.Path, selectedName, viewportRows, priorCursor)
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
	return s.load(s.Path, selectedName, viewportRows, priorCursor)
}

// Move changes the cursor by delta and keeps it visible.
func (s *State) Move(delta int, viewportRows int) {
	s.Cursor += delta
	s.clampCursor()
	s.EnsureCursorVisible(viewportRows)
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
	s.EnsureCursorVisible(viewportRows)
}

// Bottom moves the cursor to the last entry.
func (s *State) Bottom(viewportRows int) {
	if s.VisibleEntryCount() == 0 {
		s.Cursor = 0
		s.ScrollOffset = 0
		return
	}
	s.Cursor = s.VisibleEntryCount() - 1
	s.EnsureCursorVisible(viewportRows)
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
	dir := cleanPath(dirPath)
	if dir == "" {
		return false
	}
	for p, on := range s.SelectedPaths {
		if !on {
			continue
		}
		if isStrictPathDescendant(dir, cleanPath(p)) {
			return true
		}
	}
	return false
}

func isStrictPathDescendant(parent, child string) bool {
	if parent == "" || child == "" || child == parent {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	if rel == "." {
		return false
	}
	return !strings.HasPrefix(rel, "..")
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
	if s.Path == "" {
		return nil
	}

	currentPath := s.Path
	parentPath := filepath.Dir(currentPath)
	selectedName := filepath.Base(currentPath)
	if parentPath == currentPath {
		selectedName = ""
	}
	return s.NavigateTo(parentPath, selectedName, viewportRows)
}

// NavigateTo loads path after recording it in navigation history (MRU timeline).
func (s *State) NavigateTo(path string, selectedName string, viewportRows int) error {
	s.recordVisit(path)
	if err := s.load(path, selectedName, viewportRows, noIndexCursorFallback); err != nil {
		return err
	}
	if s.HistoryIndex == 0 && len(s.History) > 0 {
		s.History[0] = cleanPath(s.Path)
	}
	return nil
}

// HistoryBackward moves to an older entry in the timeline (larger HistoryIndex).
func (s *State) HistoryBackward(viewportRows int) (bool, error) {
	if len(s.History) == 0 || s.HistoryIndex >= len(s.History)-1 {
		return false, nil
	}
	nextIdx := s.HistoryIndex + 1
	target := cleanPath(s.History[nextIdx])
	if target == "" {
		return false, nil
	}
	prevIdx := s.HistoryIndex
	s.HistoryIndex = nextIdx
	if err := s.load(target, "", viewportRows, noIndexCursorFallback); err != nil {
		s.HistoryIndex = prevIdx
		return false, err
	}
	if s.HistoryIndex >= 0 && s.HistoryIndex < len(s.History) {
		s.History[s.HistoryIndex] = cleanPath(s.Path)
	}
	return true, nil
}

// HistoryForward moves to a newer entry in the timeline (smaller HistoryIndex).
func (s *State) HistoryForward(viewportRows int) (bool, error) {
	if s.HistoryIndex <= 0 {
		return false, nil
	}
	nextIdx := s.HistoryIndex - 1
	target := cleanPath(s.History[nextIdx])
	prevIdx := s.HistoryIndex
	s.HistoryIndex = nextIdx
	if err := s.load(target, "", viewportRows, noIndexCursorFallback); err != nil {
		s.HistoryIndex = prevIdx
		return false, err
	}
	if s.HistoryIndex >= 0 && s.HistoryIndex < len(s.History) {
		s.History[s.HistoryIndex] = cleanPath(s.Path)
	}
	return true, nil
}

func (s *State) recordVisit(target string) {
	target = cleanPath(target)
	if target == "" {
		return
	}
	cur := cleanPath(s.Path)
	if target == cur {
		return
	}
	var base []string
	if len(s.History) > 0 && s.HistoryIndex < len(s.History) {
		base = append([]string(nil), s.History[s.HistoryIndex:]...)
	}
	// When moving up (Parent), keep deeper timeline entries so history-back can re-walk the chain.
	if cur == "" || target == "" || !isStrictPathDescendant(target, cur) {
		base = removeStrictDescendantsOf(base, cur)
	}
	base = removePathFromSlice(base, target)
	hist := append([]string{target}, base...)
	if len(hist) > maxNavHistory {
		hist = hist[:maxNavHistory]
	}
	s.History = hist
	s.HistoryIndex = 0
}

func removePathFromSlice(slice []string, target string) []string {
	want := cleanPath(target)
	if want == "" {
		return slice
	}
	out := slice[:0]
	for _, p := range slice {
		if cleanPath(p) != want {
			out = append(out, p)
		}
	}
	return out
}

func removeStrictDescendantsOf(slice []string, parent string) []string {
	if parent == "" {
		return slice
	}
	par := cleanPath(parent)
	out := slice[:0]
	for _, item := range slice {
		pc := cleanPath(item)
		if isStrictPathDescendant(par, pc) {
			continue
		}
		out = append(out, item)
	}
	return out
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

// SetFilterCaseInsensitive configures the quick filter case behavior.
func (s *State) SetFilterCaseInsensitive(value bool, viewportRows int) {
	s.Filter.CaseInsensitive = value
	selectedName := s.currentEntryName()
	s.rebuildFilter()
	_ = s.SelectVisibleEntry(selectedName)
	s.clampCursor()
	s.EnsureCursorVisible(viewportRows)
}

// OpenFilter starts editing the panel-local quick filter.
func (s *State) OpenFilter(viewportRows int) {
	s.Filter.Editing = true
	s.rebuildFilter()
	s.clampCursor()
	s.EnsureCursorVisible(viewportRows)
}

// AcceptFilter exits editing while keeping the current filtered view.
func (s *State) AcceptFilter(viewportRows int) {
	s.Filter.Editing = false
	s.Filter.Active = s.Filter.Query != ""
	if !s.Filter.Active {
		s.Filter.results = nil
	}
	s.clampCursor()
	s.EnsureCursorVisible(viewportRows)
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
	s.EnsureCursorVisible(viewportRows)
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

func (s *State) load(path string, selectedName string, viewportRows int, indexFallback int) error {
	listing, err := localfs.ListDir(path, localfs.ListOptions{ShowHidden: s.ShowHidden})
	if err != nil {
		return err
	}

	previousPath := s.Path
	s.Path = listing.Path
	s.refreshVolumeSpace(listing.Path)
	dev, devOK := diskusage.PathDevice(listing.Path)
	s.ListingDevice = dev
	s.ListingDeviceValid = devOK
	s.Entries = listing.Entries
	s.Cursor = 0
	s.ScrollOffset = 0
	if cleanPath(previousPath) != cleanPath(listing.Path) {
		s.notifyChdir(previousPath, listing.Path)
	}
	// Activate disk-total primary sort before ApplySort so the first paint matches MC-style
	// disk ordering when cache already covers this listing (no idle timer / reconcile delay).
	if s.Sort.DiskUsageIdleSizeSort && len(s.Entries) > 0 && s.ListingFullyDiskCached() {
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
	s.EnsureCursorVisible(viewportRows)
	if len(s.History) == 0 {
		cp := cleanPath(listing.Path)
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

func cleanPath(p string) string {
	if p == "" {
		return ""
	}
	return filepath.Clean(p)
}

func (s *State) refreshVolumeSpace(forPath string) {
	avail, total, ok := fsvol.VolumeBytes(forPath)
	s.VolumeSpaceOK = ok
	if ok {
		s.VolumeAvailBytes = avail
		s.VolumeTotalBytes = total
		return
	}
	s.VolumeAvailBytes = 0
	s.VolumeTotalBytes = 0
}

func (s *State) notifyChdir(oldPath, newPath string) {
	oldC := cleanPath(oldPath)
	newC := cleanPath(newPath)
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
		if cleanPath(filepath.Dir(p)) == leftDir {
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
		if cleanPath(filepath.Dir(p)) == dir {
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
	cur := cleanPath(s.Path)
	seen := make(map[string]bool)
	out := make([]string, 0, len(s.SelectionsStripOrder))
	for _, p := range s.SelectionsStripOrder {
		if s.SelectedPaths == nil || !s.SelectedPaths[p] {
			continue
		}
		if cleanPath(filepath.Dir(p)) == cur {
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
			if cleanPath(filepath.Dir(p)) == cur {
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
	want := filepath.Clean(absPath)
	for i := 0; i < s.VisibleEntryCount(); i++ {
		entry, _, ok := s.VisibleEntry(i)
		if !ok {
			continue
		}
		if filepath.Clean(entry.Path) == want {
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
	s.EnsureCursorVisible(viewportRows)
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
// caseSensitive: when true, matching is case-sensitive (only relevant for non-shell-pattern mode).
// useShellPatterns: when true, uses filepath.Match; otherwise uses case-insensitive substring match.
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
		matched, _ := filepath.Match(pattern, name)
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
	if centerCursor {
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
	s.EnsureCursorVisible(viewportRows)
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
	s.EnsureCursorVisible(viewportRows)
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
