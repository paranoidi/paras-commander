// Package dedup owns the full-screen "find duplicates within a single directory" view.
package dedup

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/paranoidi/paras-commander/internal/quickfilter"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/hashwalk"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// Deps wires the dedup handler at app construction.
type Deps struct {
	Host       Host
	Screen     tcell.Screen
	Model      *ui.Model
	Config     config.Config
	Gitignore  *gitignore.Cache
	DiskIgnore diskusage.ShouldIgnoreFolder
}

// Host is the app shell surface dedup needs.
type Host interface {
	host.PanelNavigationHost
	EnqueueDeleteJob(paths []string, removeEmptyDirs bool)
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	DedupMenuDefinitions() []menu.Definition
	BrowserMenuDefinitions() []menu.Definition
	// Config returns the App's live config; filter case/cycle settings can change at runtime.
	Config() config.Config
}

// WakePayload wakes PollEvent when the dedup session updates.
type WakePayload struct{}

// Handler owns the dedup full-screen view.
type Handler struct {
	host       Host
	screen     tcell.Screen
	model      *ui.Model
	config     config.Config
	gitignore  *gitignore.Cache
	diskIgnore diskusage.ShouldIgnoreFolder

	session *comparepkg.DedupSession
	wake    host.WakeCoalescer
	pending *dedupPendingState

	verifyMu   sync.Mutex
	verifyGen  int
	verifyProg atomic.Int64    // files checked by the running verify
	verifyDone map[string]bool // missing paths from the latest finished verify; nil = none ready
}

// dedupPendingState carries marks/keeps/tree state across a rescan so returning
// from a compare-directories detour restores the view for paths still present.
type dedupPendingState struct {
	marked, kept                        map[string]bool
	mainCollapsed, copiesCollapsed      map[string]bool
	treeDirs, sortByWasted, ignoreEmpty bool
	prevExpandable                      []string
	mainRowID, mainRowAbsKey            string
	focusCopies                         bool
	copiesRowID, copiesRowAbsKey        string
}

// New constructs a dedup handler.
func New(d Deps) *Handler {
	return &Handler{
		host:       d.Host,
		screen:     d.Screen,
		model:      d.Model,
		config:     d.Config,
		gitignore:  d.Gitignore,
		diskIgnore: d.DiskIgnore,
	}
}

func (h *Handler) postWake() {
	h.wake.Post(h.screen, WakePayload{})
}

func (h *Handler) activePanelPath() pathloc.Path {
	if h.model.ActivePanel == ui.SecondaryPanel {
		return h.model.Secondary.Path
	}
	return h.model.Primary.Path
}

// Open brings back the kept results (via the Show/Rescan dialog) only when the
// active panel is exactly the scan's root; any other directory starts a fresh
// scan there, replacing the kept results.
func (h *Handler) Open() {
	if ui.IsAuxiliaryView(h.model.ViewMode) && h.model.ViewMode != ui.ViewDedup {
		return
	}
	path := h.activePanelPath()
	if !h.HasResults() || !path.Equal(h.model.DedupSnapshot.Root) {
		h.openRoot(path)
		return
	}
	h.model.DedupReturnDialog = dialog.DedupReturnDialogState{Open: true}
}

// HasResults reports whether a finished scan's results are kept in memory.
func (h *Handler) HasResults() bool {
	return h.session == nil && h.model.DedupSnapshot.Phase == comparepkg.DedupDone
}

// ShowKept switches to the dedup view over the kept results; no-op without any.
func (h *Handler) ShowKept() {
	if !h.HasResults() {
		return
	}
	h.model.ViewMode = ui.ViewDedup
	h.model.MenuDefinitions = h.host.DedupMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndexDedup()
}

// VerifyAndShowKept shows the kept results at once and re-stats every file in
// the background; PollUpdates shows progress in the title and, when the check
// finishes, drops vanished files.
func (h *Handler) VerifyAndShowKept() {
	if !h.HasResults() {
		return
	}
	h.ShowKept()
	var paths []pathloc.Path
	for _, g := range h.model.DedupSnapshot.Groups {
		for _, f := range g.Files {
			paths = append(paths, f.Abs)
		}
	}
	h.verifyMu.Lock()
	h.verifyGen++
	gen := h.verifyGen
	h.verifyDone = nil
	h.verifyMu.Unlock()
	h.verifyProg.Store(0)
	h.model.DedupView.RecheckDone, h.model.DedupView.RecheckTotal = 0, len(paths)
	h.syncDedupList()
	h.ensureSelectionVisible(0)
	go func() {
		missing := map[string]bool{}
		last := time.Now()
		for i, p := range paths {
			if _, err := fsbackend.Default().Stat(context.Background(), p); err != nil {
				missing[p.String()] = true
			}
			h.verifyProg.Store(int64(i + 1))
			if time.Since(last) >= 50*time.Millisecond {
				last = time.Now()
				h.postWake()
			}
		}
		h.verifyMu.Lock()
		if h.verifyGen == gen {
			h.verifyDone = missing
		}
		h.verifyMu.Unlock()
		h.postWake()
	}()
}

// applyVerified publishes check progress and consumes a finished result; true
// when the view changed.
func (h *Handler) applyVerified() bool {
	st := &h.model.DedupView
	h.verifyMu.Lock()
	missing := h.verifyDone
	h.verifyDone = nil
	h.verifyMu.Unlock()
	if missing == nil {
		if st.RecheckTotal > 0 && h.HasResults() {
			st.RecheckDone = int(h.verifyProg.Load())
			return true
		}
		return false
	}
	st.RecheckDone, st.RecheckTotal = 0, 0
	if !h.HasResults() {
		return false
	}
	for abs := range missing {
		delete(st.Kept, abs)
	}
	h.model.DedupSnapshot = h.model.DedupSnapshot.WithoutPaths(missing)
	if len(h.model.DedupSnapshot.Groups) == 0 {
		h.Close()
		h.host.SetTransientMessage("Duplicates: all files gone", ui.MessageUrgencyInfo)
		return true
	}
	// Re-apply marks for survivors only (WithoutPaths drops whole groups too).
	oldMarked := st.Marked
	st.Marked, st.MarkedCount, st.MarkedReclaimBytes = map[string]bool{}, 0, 0
	for _, g := range h.model.DedupSnapshot.Groups {
		for _, f := range g.Files {
			if abs := f.Abs.String(); oldMarked[abs] {
				h.setMark(abs, g.Size, true)
			}
		}
	}
	h.syncDedupList()
	h.ensureSelectionVisible(0)
	return true
}

// Leave returns to the browser but keeps the results for Open/ShowKept.
func (h *Handler) Leave() {
	if h.model.ViewMode == ui.ViewDedup {
		h.model.ViewMode = ui.ViewBrowser
		h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
		h.model.Menu.ActiveMenu = menu.DefaultIndex()
	}
}

// openRoot cancels any previous scan and starts a new one on root. Walk options
// (hidden files, volume gate) still come from the active panel — panels cannot
// navigate while the dedup or compare view is open.
func (h *Handler) openRoot(root pathloc.Path) {
	h.Close()

	p := &h.model.Primary
	if h.model.ActivePanel == ui.SecondaryPanel {
		p = &h.model.Secondary
	}
	if root.IsZero() {
		h.host.SetTransientMessage("Find duplicates: panel needs a path", ui.MessageUrgencyError)
		return
	}
	if root.IsRemote() {
		h.host.SetTransientMessage("Find duplicates: remote paths not supported", ui.MessageUrgencyError)
		return
	}

	h.model.DedupView = ui.DedupViewState{
		Marked:                map[string]bool{},
		Kept:                  map[string]bool{},
		IgnoreEmpty:           true,
		TreeDirs:              true,
		DirsCollapsePending:   true,
		GroupsCollapsePending: true,
	}
	h.model.DedupProgressDialog = dialog.DedupProgressDialogState{Open: true, ButtonFocus: 0}

	volGate := diskusage.ListingVolumeGate{}
	if h.config.Compare.StayOnVolumeDefault {
		volGate = diskusage.ListingVolumeGate{
			Enabled: p.ListingDeviceValid,
			RefDev:  p.ListingDevice,
			Valid:   p.ListingDeviceValid,
		}
	}
	hs := hashwalk.FromCompareConfig(h.config.Compare, h.diskIgnore, volGate)

	h.session = comparepkg.StartDedup(context.Background(), root, comparepkg.DedupOptions{
		Walk: comparepkg.WalkOptions{
			ShowHidden:    p.ShowHidden,
			Gitignore:     h.gitignore,
			ShouldSkipDir: hs.ShouldSkip,
		},
		HashWorkers:       hs.HashWorkers,
		ReadBuffer:        hs.ReadBuffer,
		MaxHashBytes:      hs.MaxHashBytes,
		ConfirmHashBytes:  h.config.Dedup.HashConfirmBytes,
		FileProgressBytes: h.config.Dedup.FileProgressBytes,
		ChunkBytes:        h.config.Dedup.ChunkBytes,
		CacheFile:         comparepkg.DefaultHashCachePath(),
		OnUpdate:          func(_ comparepkg.DedupSnapshot) { h.postWake() },
	})
	h.model.DedupSnapshot = h.session.Snapshot()
}

// Close cancels the scan and returns to the browser.
func (h *Handler) Close() {
	h.pending = nil
	h.verifyMu.Lock()
	h.verifyGen++
	h.verifyDone = nil
	h.verifyMu.Unlock()
	if h.session != nil {
		h.session.Close()
		h.session = nil
	}
	if h.model.ViewMode == ui.ViewDedup {
		h.model.ViewMode = ui.ViewBrowser
		h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
		h.model.Menu.ActiveMenu = menu.DefaultIndex()
	}
	h.model.DedupProgressDialog = dialog.DedupProgressDialogState{}
	h.model.DedupReturnDialog = dialog.DedupReturnDialogState{}
	h.model.DedupView = ui.DedupViewState{}
	h.model.DedupSnapshot = comparepkg.DedupSnapshot{}
	h.model.DedupList = nil
	h.model.DedupCopiesList = nil
}

// PollUpdates applies the latest session snapshot. Returns true when the UI should repaint.
func (h *Handler) PollUpdates(_ WakePayload) bool {
	_ = h.wake.Take()
	if h.applyVerified() {
		return true
	}
	if h.session == nil {
		return false
	}
	snap := h.session.Snapshot()
	h.model.DedupSnapshot = snap

	switch snap.Phase {
	case comparepkg.DedupDone:
		h.enterResultsView()
		return true
	case comparepkg.DedupError:
		msg := snap.Err
		if msg == "" {
			msg = "Find duplicates failed"
		}
		h.finishScan(msg, ui.MessageUrgencyError)
		return true
	case comparepkg.DedupCanceled:
		h.finishScan("", ui.MessageUrgencyInfo)
		return true
	default:
		return true
	}
}

func (h *Handler) enterResultsView() {
	h.session = nil
	snap := h.model.DedupSnapshot.WithTrimmedDisplayRoot()
	h.model.DedupSnapshot = snap
	if snap.DisplayRootTrimmed() {
		h.host.SetTransientMessage("Duplicates view re-rooted", ui.MessageUrgencyInfo)
	}
	h.model.DedupProgressDialog = dialog.DedupProgressDialogState{}
	h.ShowKept()
	h.applyPending(snap)
	h.syncDedupList()
	h.restorePendingCursor()
	h.ensureSelectionVisible(0)
}

// applyPending restores tree toggles, collapse maps, and marks/keeps captured by
// ReopenPreservingState, pruning entries no longer present in the new snapshot.
// No-op when no rescan-with-state is pending.
func (h *Handler) applyPending(snap comparepkg.DedupSnapshot) {
	p := h.pending
	if p == nil {
		return
	}
	st := &h.model.DedupView
	st.TreeDirs = p.treeDirs
	st.SortByWasted = p.sortByWasted
	st.IgnoreEmpty = p.ignoreEmpty
	st.Main.Collapsed = p.mainCollapsed
	st.Copies.Collapsed = p.copiesCollapsed
	// Only the restored mode keeps its collapse state; the other mode still gets
	// its collapse-all first build. Nodes new in the rescan come up collapsed.
	if p.treeDirs {
		st.DirsCollapsePending = false
	} else {
		st.GroupsCollapsePending = false
	}
	ui.DedupCollapseNewIDs(&st.Main, p.prevExpandable, ui.DedupExpandableIDs(snap, *st))
	for _, g := range snap.Groups {
		for _, f := range g.Files {
			abs := f.Abs.String()
			if p.marked[abs] {
				h.setMark(abs, g.Size, true)
			}
			if p.kept[abs] {
				st.Kept[abs] = true
			}
		}
	}
}

// restorePendingCursor re-locates the main cursor by its pre-rescan row ID (mark
// key fallback) and drops the pending state. Runs after syncDedupList.
func (h *Handler) restorePendingCursor() {
	p := h.pending
	if p == nil {
		return
	}
	h.pending = nil
	st := &h.model.DedupView
	if i := ui.DedupRowIndexByID(h.model.DedupList, p.mainRowID); i >= 0 {
		st.Main.Selected = i
	} else if i := ui.DedupRowIndexByID(h.model.DedupList, p.mainRowAbsKey); i >= 0 {
		st.Main.Selected = i
	}
	h.syncCopies()
	if i := ui.DedupRowIndexByID(h.model.DedupCopiesList, p.copiesRowID); i >= 0 {
		st.Copies.Selected = i
	} else if i := ui.DedupRowIndexByID(h.model.DedupCopiesList, p.copiesRowAbsKey); i >= 0 {
		st.Copies.Selected = i
	}
	if p.focusCopies && len(h.model.DedupCopiesList) > 0 {
		st.FocusCopies = true
	}
}

func (h *Handler) finishScan(message string, urgency ui.MessageUrgency) {
	h.Close()
	if message != "" {
		h.host.SetTransientMessage(message, urgency)
	}
}

// Confirm resumes hashing after the hash-bytes confirmation pause.
func (h *Handler) Confirm() {
	if h.session != nil {
		h.session.Confirm()
	}
}

// Refresh re-runs the scan on the view's root.
func (h *Handler) Refresh() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.openRoot(h.model.DedupSnapshot.Root)
}

// ReopenPreservingState re-scans the previous snapshot root and, once the new
// scan completes, restores marks, keeps, collapse state, and the cursor for
// entries still present in the new results. Used as the return hook when
// compare-directories was opened from this view.
func (h *Handler) ReopenPreservingState() {
	root := h.model.DedupSnapshot.Root
	if root.IsZero() {
		h.Open()
		return
	}
	st := h.model.DedupView
	p := dedupPendingState{
		marked:          st.Marked,
		kept:            st.Kept,
		mainCollapsed:   st.Main.Collapsed,
		copiesCollapsed: st.Copies.Collapsed,
		treeDirs:        st.TreeDirs,
		sortByWasted:    st.SortByWasted,
		ignoreEmpty:     st.IgnoreEmpty,
		prevExpandable:  ui.DedupExpandableIDs(h.model.DedupSnapshot, st),
	}
	if row, ok := h.paneRow(&h.model.DedupView.Main, h.model.DedupList); ok {
		p.mainRowID, p.mainRowAbsKey = row.ID, row.Value.AbsKey
	}
	p.focusCopies = st.FocusCopies
	if row, ok := h.paneRow(&st.Copies, h.model.DedupCopiesList); ok {
		p.copiesRowID, p.copiesRowAbsKey = row.ID, row.Value.AbsKey
	}
	// Capture above happens before openRoot (its Close prelude wipes the model);
	// pending is set after (the same prelude clears h.pending).
	h.openRoot(root)
	if h.session == nil {
		return // openRoot refused; message already shown
	}
	h.pending = &p
}

// CompareDirsFromSelection resolves the directory pair for compare-directories:
// primary = the main-pane selected file's directory, secondary = the directory
// under the copies-pane cursor (dir row → scan root + rel, file row → parent).
// Emits a transient message and returns ok=false when no pair can be formed.
func (h *Handler) CompareDirsFromSelection() (primary, secondary pathloc.Path, ok bool) {
	st := &h.model.DedupView
	mainRow, okMain := h.paneRow(&st.Main, h.model.DedupList)
	if !okMain || mainRow.Value.Kind != ui.DedupRowFile {
		h.host.SetTransientMessage("Compare: select a duplicate file first", ui.MessageUrgencyInfo)
		return pathloc.Path{}, pathloc.Path{}, false
	}
	copyRow, okCopy := h.paneRow(&st.Copies, h.model.DedupCopiesList)
	if !okCopy {
		h.host.SetTransientMessage("Compare: no copies to compare against", ui.MessageUrgencyInfo)
		return pathloc.Path{}, pathloc.Path{}, false
	}
	primary = mainRow.Value.File.Abs.Parent()
	if copyRow.Value.Kind == ui.DedupRowDir {
		dir, err := comparepkg.JoinRel(h.model.DedupSnapshot.EffectiveDisplayRoot(), copyRow.Value.DirRel)
		if err != nil {
			h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyError)
			return pathloc.Path{}, pathloc.Path{}, false
		}
		secondary = dir
	} else {
		secondary = copyRow.Value.File.Abs.Parent()
	}
	return primary, secondary, true
}

// EnsureSelectionVisible keeps both panes' cursor rows visible for visibleRows height.
func (h *Handler) EnsureSelectionVisible(visibleRows int) {
	h.ensureSelectionVisible(visibleRows)
}

// applyDedupCollapsePending collapses every expandable node the first time each tree
// mode's results are built. Uses SetCollapsed so switching modes preserves the other
// mode's collapse map entries.
func (h *Handler) applyDedupCollapsePending() {
	st := &h.model.DedupView
	if h.model.DedupSnapshot.Phase != comparepkg.DedupDone {
		return
	}
	if st.TreeDirs && st.DirsCollapsePending {
		for _, id := range ui.DedupExpandableIDs(h.model.DedupSnapshot, *st) {
			st.Main.SetCollapsed(id, true)
		}
		st.DirsCollapsePending = false
	} else if !st.TreeDirs && st.GroupsCollapsePending {
		for _, id := range ui.DedupExpandableIDs(h.model.DedupSnapshot, *st) {
			st.Main.SetCollapsed(id, true)
		}
		st.GroupsCollapsePending = false
	}
}

// syncDedupList rebuilds the main tree rows and, from the main selection, the
// copies pane rows.
func (h *Handler) syncDedupList() {
	st := &h.model.DedupView
	h.applyDedupCollapsePending()
	h.model.DedupList, st.IgnoredEmptyCount = ui.DedupRowsFromSnapshot(h.model.DedupSnapshot, *st)
	h.refilterPane(&st.Main, h.model.DedupList)
	h.syncCopies()
}

// syncCopies rebuilds the copies pane from the main pane's selected row,
// preserving the copies cursor by row ID when possible.
func (h *Handler) syncCopies() {
	st := &h.model.DedupView
	var prevID string
	if row, ok := h.paneRow(&st.Copies, h.model.DedupCopiesList); ok {
		prevID = row.ID
	}
	sel, _ := h.paneRow(&st.Main, h.model.DedupList)
	h.model.DedupCopiesList = ui.DedupCopyRows(h.model.DedupSnapshot, sel, st.Copies.Collapsed)
	h.refilterPane(&st.Copies, h.model.DedupCopiesList)
	if i := ui.DedupRowIndexByID(h.model.DedupCopiesList, prevID); i >= 0 {
		st.Copies.Selected = i
	} else {
		st.Copies.Selected = 0
		st.Copies.ListScroll = 0
	}
	if len(h.model.DedupCopiesList) == 0 {
		st.FocusCopies = false
	}
}

func (h *Handler) paneRow(p *ui.DedupPane, rows []ui.DedupRow) (ui.DedupRow, bool) {
	if p.Selected < 0 || p.Selected >= len(rows) {
		return ui.DedupRow{}, false
	}
	return rows[p.Selected], true
}

// focusedPane returns the pane the cursor keys act on plus its rows.
func (h *Handler) focusedPane() (*ui.DedupPane, []ui.DedupRow) {
	st := &h.model.DedupView
	if st.FocusCopies {
		return &st.Copies, h.model.DedupCopiesList
	}
	return &st.Main, h.model.DedupList
}

// selectedRow returns the focused pane's row under the cursor.
func (h *Handler) selectedRow() (ui.DedupRow, bool) {
	pane, rows := h.focusedPane()
	return h.paneRow(pane, rows)
}

// MoveToAdjacentDir moves the focused pane's cursor to the previous (delta -1)
// or next (delta +1) visible directory row. File rows are skipped. When no such
// directory exists the cursor stays put (no wrap).
func (h *Handler) MoveToAdjacentDir(delta int) {
	if delta != -1 && delta != 1 {
		return
	}
	pane, rows := h.focusedPane()
	if len(rows) == 0 {
		return
	}
	for i := pane.Selected + delta; i >= 0 && i < len(rows); i += delta {
		if rows[i].Value.Kind == ui.DedupRowDir {
			pane.Selected = i
			if !h.model.DedupView.FocusCopies {
				h.syncCopies()
			}
			return
		}
	}
}

// MoveSelection moves the focused pane's cursor by delta rows (clamped). Main
// pane moves refresh the copies pane.
func (h *Handler) MoveSelection(delta int) {
	pane, rows := h.focusedPane()
	if len(rows) == 0 {
		return
	}
	pane.Selected = min(max(pane.Selected+delta, 0), len(rows)-1)
	if !h.model.DedupView.FocusCopies {
		h.syncCopies()
	}
}

// FocusedFilter returns a copy of the focused pane's quick filter state.
func (h *Handler) FocusedFilter() quickfilter.Filter {
	pane, _ := h.focusedPane()
	return pane.Filter
}

// applyFilter sets the focused pane's query and jumps its cursor to the best match.
// A main-pane jump refreshes the copies pane.
func (h *Handler) applyFilter(query string) {
	pane, rows := h.focusedPane()
	if cur, ok := pane.Filter.Apply(query, dedupRowNames(rows), h.filterOptions()); ok {
		h.setSelected(pane, cur)
	}
}

// refilterPane re-ranks an open filter after pane's rows were rebuilt, since
// match results hold row indices.
func (h *Handler) refilterPane(pane *ui.DedupPane, rows []ui.DedupRow) {
	if pane.Filter.Query != "" {
		pane.Filter.Rebuild(dedupRowNames(rows), h.filterOptions())
	}
}

func dedupRowNames(rows []ui.DedupRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Value.Display
	}
	return names
}

// setSelected moves pane's cursor; a main-pane move refreshes the copies pane.
func (h *Handler) setSelected(pane *ui.DedupPane, index int) {
	pane.Selected = index
	if !h.model.DedupView.FocusCopies {
		h.syncCopies()
	}
}

// AppendFilterRune types r into the focused pane's quick filter.
func (h *Handler) AppendFilterRune(r rune) {
	pane, _ := h.focusedPane()
	h.applyFilter(pane.Filter.InsertRune(r))
}

// BackspaceFilter deletes the rune before the filter caret.
func (h *Handler) BackspaceFilter() {
	pane, _ := h.focusedPane()
	if next, changed := pane.Filter.Backspace(); changed {
		h.applyFilter(next)
	}
}

// ClearFilter empties the query while the filter stays open.
func (h *Handler) ClearFilter() {
	h.applyFilter("")
}

// CancelFilter closes the focused pane's quick filter.
func (h *Handler) CancelFilter() {
	pane, _ := h.focusedPane()
	pane.Filter = quickfilter.Filter{}
}

// CycleFilterMatch steps the focused pane's cursor through filter matches, or
// moves plainly when nothing matches.
func (h *Handler) CycleFilterMatch(delta int) {
	pane, _ := h.focusedPane()
	if cur, ok := pane.Filter.Cycle(pane.Selected, delta, h.filterOptions()); ok {
		h.setSelected(pane, cur)
		return
	}
	h.MoveSelection(delta)
}

// SelectEdge moves the focused pane's cursor to the first or last row.
func (h *Handler) SelectEdge(last bool) {
	pane, rows := h.focusedPane()
	if len(rows) == 0 {
		return
	}
	if last {
		pane.Selected = len(rows) - 1
	} else {
		pane.Selected = 0
	}
	if !h.model.DedupView.FocusCopies {
		h.syncCopies()
	}
}

// SwitchPane cycles focus (Tab): main tree -> copies pane (if non-empty) -> browse panel (Dirs
// view, results ready) -> main tree. FocusCopies keeps naming the source pane while the browse
// panel is focused.
func (h *Handler) SwitchPane() {
	st := &h.model.DedupView
	hasPanel := st.TreeDirs && h.model.DedupSnapshot.Phase == comparepkg.DedupDone
	st.Main.Filter, st.Copies.Filter = quickfilter.Filter{}, quickfilter.Filter{}
	switch {
	case st.FocusPanel:
		st.FocusPanel, st.FocusCopies = false, false
	case !st.FocusCopies && len(h.model.DedupCopiesList) > 0:
		st.FocusCopies = true
	case hasPanel:
		st.FocusPanel = true
	default:
		st.FocusCopies = false
	}
}

// resyncPreservingCursor rebuilds the visible rows and re-locates the main
// cursor by node ID (with the row's mark key as fallback across tree-mode
// switches).
func (h *Handler) resyncPreservingCursor() {
	st := &h.model.DedupView
	var id, absKey string
	if row, ok := h.paneRow(&st.Main, h.model.DedupList); ok {
		id, absKey = row.ID, row.Value.AbsKey
	}
	h.applyDedupCollapsePending()
	h.model.DedupList, st.IgnoredEmptyCount = ui.DedupRowsFromSnapshot(h.model.DedupSnapshot, *st)
	if i := ui.DedupRowIndexByID(h.model.DedupList, id); i >= 0 {
		st.Main.Selected = i
	} else if i := ui.DedupRowIndexByID(h.model.DedupList, absKey); i >= 0 {
		st.Main.Selected = i
	}
	h.syncCopies()
	h.ensureSelectionVisible(0)
}

// activeGroups returns the groups the view operates on (ignore-empty applied).
func (h *Handler) activeGroups() []comparepkg.DedupGroup {
	return ui.DedupActiveGroups(h.model.DedupSnapshot, h.model.DedupView.IgnoreEmpty)
}

// ToggleSortOrder flips between order-by-path and most-space-wasted (groups tree).
func (h *Handler) ToggleSortOrder() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.model.DedupView.SortByWasted = !h.model.DedupView.SortByWasted
	h.resyncPreservingCursor()
}

// ToggleTreeMode switches between the duplicate-groups tree and the directory
// hierarchy tree. Marks and collapse state are keyed by stable IDs and survive.
func (h *Handler) ToggleTreeMode() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.model.DedupView.TreeDirs = !h.model.DedupView.TreeDirs
	h.resyncPreservingCursor()
}

// resyncFocused rebuilds the focused pane after a collapse-state change: the
// copies pane rebuilds alone; the main pane rebuilds everything.
func (h *Handler) resyncFocused() {
	st := &h.model.DedupView
	if st.FocusCopies {
		sel, _ := h.paneRow(&st.Main, h.model.DedupList)
		prev, _ := h.paneRow(&st.Copies, h.model.DedupCopiesList)
		h.model.DedupCopiesList = ui.DedupCopyRows(h.model.DedupSnapshot, sel, st.Copies.Collapsed)
		if i := ui.DedupRowIndexByID(h.model.DedupCopiesList, prev.ID); i >= 0 {
			st.Copies.Selected = i
		}
		h.ensureSelectionVisible(0)
		return
	}
	h.resyncPreservingCursor()
}

// DescendFromSelection handles Right on tree rows. Collapsed directory rows
// expand in place and keep the cursor on the folder; expanded directory rows
// descend into the subtree and land on the first file row in depth-first order,
// expanding collapsed folders along the way without collapsing expanded ones.
// Group headers (file rows with child copies) still toggle expand/collapse.
func (h *Handler) DescendFromSelection() {
	row, ok := h.selectedRow()
	if !ok || !row.HasChildren {
		return
	}
	if row.Value.Kind == ui.DedupRowDir {
		h.descendIntoDir()
		return
	}
	h.toggleExpandableNode()
}

func (h *Handler) toggleExpandableNode() {
	row, ok := h.selectedRow()
	if !ok || !row.HasChildren {
		return
	}
	pane, _ := h.focusedPane()
	expanding := pane.Collapsed[row.ID]
	pane.SetCollapsed(row.ID, !expanding)
	h.resyncFocused()
}

// descendIntoDir expands a collapsed selected directory in place, or when the
// directory is already expanded moves the cursor to the first file descendant
// in flattened tree order. Collapsed intermediate directories on the path are
// expanded; already-expanded directories are left open. When no file exists
// under the directory, the cursor moves to the first direct child row.
func (h *Handler) descendIntoDir() {
	pane, rows := h.focusedPane()
	row, ok := h.paneRow(pane, rows)
	if !ok || row.Value.Kind != ui.DedupRowDir || !row.HasChildren {
		return
	}
	if pane.Collapsed[row.ID] {
		pane.SetCollapsed(row.ID, false)
		h.resyncFocused()
		return
	}
	for {
		dirIdx := pane.Selected
		row, ok = h.paneRow(pane, rows)
		if !ok || row.Value.Kind != ui.DedupRowDir {
			break
		}
		depth := row.Depth
		end := ui.DedupSubtreeEndIndex(rows, dirIdx)
		expandedChild := false
		for i := dirIdx + 1; i < end; i++ {
			child := rows[i]
			if child.Value.Kind == ui.DedupRowFile {
				pane.Selected = i
				if !h.model.DedupView.FocusCopies {
					h.syncCopies()
				}
				return
			}
			if child.Value.Kind == ui.DedupRowDir && child.HasChildren && pane.Collapsed[child.ID] {
				pane.SetCollapsed(child.ID, false)
				h.resyncFocused()
				pane, rows = h.focusedPane()
				expandedChild = true
				break
			}
		}
		if expandedChild {
			continue
		}
		if dirIdx+1 < end && rows[dirIdx+1].Depth == depth+1 {
			pane.Selected = dirIdx + 1
			if !h.model.DedupView.FocusCopies {
				h.syncCopies()
			}
		}
		return
	}
}

// CollapseOrParent collapses the expanded node under the cursor, or moves the
// cursor to the row's parent when it is a leaf or already collapsed.
func (h *Handler) CollapseOrParent() {
	row, ok := h.selectedRow()
	if !ok {
		return
	}
	pane, rows := h.focusedPane()
	if row.HasChildren && row.Expanded {
		pane.SetCollapsed(row.ID, true)
		h.resyncFocused()
		return
	}
	for i := pane.Selected - 1; i >= 0; i-- {
		if rows[i].Depth < row.Depth {
			pane.Selected = i
			break
		}
	}
	h.ensureSelectionVisible(0)
}

// CollapseAll collapses every expandable node in the focused pane; the cursor
// lands on the nearest top-level ancestor of the previously selected row.
func (h *Handler) CollapseAll() {
	st := &h.model.DedupView
	pane, rows := h.focusedPane()
	ancestorID := ""
	if row, ok := h.paneRow(pane, rows); ok {
		ancestorID = row.ID
		if row.Depth > 0 {
			for i := pane.Selected - 1; i >= 0; i-- {
				if rows[i].Depth == 0 {
					ancestorID = rows[i].ID
					break
				}
			}
		}
	}
	var ids []string
	if st.FocusCopies {
		sel, _ := h.paneRow(&st.Main, h.model.DedupList)
		ids = ui.DedupCopyExpandableIDs(h.model.DedupSnapshot, sel)
	} else {
		ids = ui.DedupExpandableIDs(h.model.DedupSnapshot, *st)
	}
	pane.Collapsed = map[string]bool{}
	for _, id := range ids {
		pane.Collapsed[id] = true
	}
	h.resyncFocused()
	pane, rows = h.focusedPane()
	if i := ui.DedupRowIndexByID(rows, ancestorID); i >= 0 {
		pane.Selected = i
	}
	h.ensureSelectionVisible(0)
}

// ExpandAll expands the focused pane like the main file list's full expand-all:
// down to panel.MaxExpandAllShallowDepth levels, and fewer when the result would
// exceed panel.MaxExpandAllTotalRows rows. Nodes at the cutoff stay collapsed.
func (h *Handler) ExpandAll() {
	st := &h.model.DedupView
	pane, _ := h.focusedPane()
	var full []ui.DedupRow
	if st.FocusCopies {
		sel, _ := h.paneRow(&st.Main, h.model.DedupList)
		full = ui.DedupCopyRows(h.model.DedupSnapshot, sel, nil)
	} else {
		view := *st
		view.Main.Collapsed = nil
		full, _ = ui.DedupRowsFromSnapshot(h.model.DedupSnapshot, view)
	}
	limit := panel.MaxExpandAllShallowDepth
	for ; limit > 0; limit-- {
		n := 0
		for _, r := range full {
			if r.Depth <= limit {
				n++
			}
		}
		if n <= panel.MaxExpandAllTotalRows {
			break
		}
	}
	pane.Collapsed = nil
	for _, r := range full {
		if r.HasChildren && r.Depth >= limit {
			if pane.Collapsed == nil {
				pane.Collapsed = map[string]bool{}
			}
			pane.Collapsed[r.ID] = true
		}
	}
	switch {
	case limit < panel.MaxExpandAllShallowDepth:
		h.host.SetTransientMessage(fmt.Sprintf("Expand all stopped: too many entries (limit %d)", panel.MaxExpandAllTotalRows), ui.MessageUrgencyInfo)
	case pane.Collapsed != nil:
		h.toastExpandDepthLimit()
	}
	h.resyncFocused()
}

func (h *Handler) toastExpandDepthLimit() {
	h.host.SetTransientMessage(fmt.Sprintf("Expand all is limited to depth %d", panel.MaxExpandAllShallowDepth), ui.MessageUrgencyInfo)
}

// CollapseLevel collapses the focused pane by one level like the main list's
// panel.tree-collapse-all: every expanded node with no expanded descendant (the
// deepest open point of its branch) is collapsed. The cursor stays on its row,
// or moves to its nearest surviving ancestor.
func (h *Handler) CollapseLevel() {
	pane, rows := h.focusedPane()
	var keep []string
	if row, ok := h.paneRow(pane, rows); ok {
		keep = append(keep, row.ID)
		for i, d := pane.Selected-1, row.Depth; i >= 0 && d > 0; i-- {
			if rows[i].Depth < d {
				keep = append(keep, rows[i].ID)
				d = rows[i].Depth
			}
		}
	}
	changed := false
	for i, r := range rows {
		if !r.HasChildren || !r.Expanded {
			continue
		}
		frontier := true
		for _, c := range rows[i+1:] {
			if c.Depth <= r.Depth {
				break
			}
			if c.HasChildren && c.Expanded {
				frontier = false
				break
			}
		}
		if frontier {
			pane.SetCollapsed(r.ID, true)
			changed = true
		}
	}
	if !changed {
		return
	}
	h.resyncFocused()
	pane, rows = h.focusedPane()
	for _, id := range keep {
		if i := ui.DedupRowIndexByID(rows, id); i >= 0 {
			pane.Selected = i
			break
		}
	}
	h.ensureSelectionVisible(0)
}

// ExpandLevel expands the focused pane by one level like the main list's
// panel.tree-expand-all-shallow: every collapsed node at the shallowest depth
// that has one. Stops at panel.MaxExpandAllShallowDepth with an info toast.
func (h *Handler) ExpandLevel() {
	pane, rows := h.focusedPane()
	depth := -1
	for _, r := range rows {
		if r.HasChildren && !r.Expanded && (depth < 0 || r.Depth < depth) {
			depth = r.Depth
		}
	}
	if depth < 0 {
		return
	}
	if depth >= panel.MaxExpandAllShallowDepth {
		h.toastExpandDepthLimit()
		return
	}
	for _, r := range rows {
		if r.HasChildren && !r.Expanded && r.Depth == depth {
			pane.SetCollapsed(r.ID, false)
		}
	}
	h.resyncFocused()
}

// ToggleIgnoreEmpty flips whether zero-byte duplicate groups are hidden.
func (h *Handler) ToggleIgnoreEmpty() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	st := &h.model.DedupView
	prevIDs := ui.DedupExpandableIDs(h.model.DedupSnapshot, *st)
	st.IgnoreEmpty = !st.IgnoreEmpty
	ui.DedupCollapseNewIDs(&st.Main, prevIDs, ui.DedupExpandableIDs(h.model.DedupSnapshot, *st))
	h.model.MenuDefinitions = h.host.DedupMenuDefinitions()
	h.syncDedupList()
	h.ensureSelectionVisible(0)
}

func (h *Handler) ensureSelectionVisible(visibleRows int) {
	st := &h.model.DedupView
	st.Main.EnsureSelectionVisible(len(h.model.DedupList), visibleRows)
	st.Copies.EnsureSelectionVisible(len(h.model.DedupCopiesList), visibleRows)
}

// setMark adds/removes one file's delete mark, keeping count and reclaim bytes in sync.
func (h *Handler) setMark(absKey string, size int64, on bool) {
	st := &h.model.DedupView
	if st.Marked == nil {
		st.Marked = map[string]bool{}
	}
	switch {
	case on && !st.Marked[absKey]:
		st.Marked[absKey] = true
		st.MarkedCount++
		st.MarkedReclaimBytes += size
	case !on && st.Marked[absKey]:
		delete(st.Marked, absKey)
		st.MarkedCount--
		st.MarkedReclaimBytes -= size
	}
}

// ToggleMark flips the delete mark on the selected file. Directory rows use
// SelectToggleAndAdvance (Insert) for folder bulk mark in both panes.
func (h *Handler) ToggleMark() {
	row, ok := h.selectedRow()
	if !ok || row.Value.Kind != ui.DedupRowFile {
		return
	}
	st := h.model.DedupView
	h.setMark(row.Value.AbsKey, row.Value.Size, !st.Marked[row.Value.AbsKey])
}

// SelectToggleAndAdvance toggles delete marks on the focused row and moves the
// cursor. In the file-tree pane, directory rows recursively mark or clear all
// duplicate files under the folder, then skip to the next directory row. The
// copies pane uses the same folder behavior for copy files under the folder.
func (h *Handler) SelectToggleAndAdvance() {
	row, ok := h.selectedRow()
	if !ok {
		return
	}
	st := &h.model.DedupView
	if st.FocusCopies && row.Value.Kind == ui.DedupRowDir {
		h.toggleCopiesFolderMarks(row)
		_, rows := h.focusedPane()
		next := ui.DedupNextDirRowIndex(rows, st.Copies.Selected)
		if next >= len(rows) {
			next = len(rows) - 1
		}
		st.Copies.Selected = next
		return
	}
	if !st.FocusCopies && row.Value.Kind == ui.DedupRowDir {
		h.toggleMainFolderMarks(row)
		_, rows := h.focusedPane()
		next := ui.DedupNextDirRowIndex(rows, st.Main.Selected)
		if next >= len(rows) {
			next = len(rows) - 1
		}
		st.Main.Selected = next
		return
	}
	if row.Value.Kind != ui.DedupRowFile {
		return
	}
	if st.Kept[row.Value.AbsKey] {
		return
	}
	h.setMark(row.Value.AbsKey, row.Value.Size, !st.Marked[row.Value.AbsKey])
	h.MoveSelection(1)
}

// ToggleCopiesPaneSelectAll marks every copy-pane file for the current main
// selection, or clears those marks when they are all already marked. No-op when
// the copies pane is not focused or has no rows.
func (h *Handler) ToggleCopiesPaneSelectAll() {
	st := &h.model.DedupView
	if !st.FocusCopies || len(h.model.DedupCopiesList) == 0 {
		return
	}
	mainSel, ok := h.paneRow(&st.Main, h.model.DedupList)
	if !ok {
		return
	}
	files := ui.DedupCopyPaneFiles(h.model.DedupSnapshot, mainSel)
	if len(files) == 0 {
		return
	}
	unmark := true
	for _, f := range files {
		if !st.Marked[f.Abs.String()] {
			unmark = false
			break
		}
	}
	gi := mainSel.Value.GroupIdx
	size := int64(0)
	if gi >= 0 && gi < len(h.model.DedupSnapshot.Groups) {
		size = h.model.DedupSnapshot.Groups[gi].Size
	}
	for _, f := range files {
		abs := f.Abs.String()
		if !unmark && st.Kept[abs] {
			continue
		}
		h.setMark(abs, size, !unmark)
	}
}

func (h *Handler) toggleMainFolderMarks(dirRow ui.DedupRow) {
	byGroup := ui.DedupSnapshotFilesUnderDir(h.model.DedupSnapshot, dirRow.Value.DirRel)
	if len(byGroup) == 0 {
		return
	}
	st := h.model.DedupView
	clear := false
	for _, files := range byGroup {
		for _, f := range files {
			if st.Marked[f.Abs.String()] {
				clear = true
				break
			}
		}
		if clear {
			break
		}
	}
	for gi, files := range byGroup {
		size := h.model.DedupSnapshot.Groups[gi].Size
		for _, f := range files {
			abs := f.Abs.String()
			if !clear && st.Kept[abs] {
				continue
			}
			h.setMark(abs, size, !clear)
		}
	}
}

func (h *Handler) toggleCopiesFolderMarks(dirRow ui.DedupRow) {
	mainSel, ok := h.paneRow(&h.model.DedupView.Main, h.model.DedupList)
	if !ok {
		return
	}
	files := ui.DedupCopyFilesUnderDir(h.model.DedupSnapshot, mainSel, dirRow.Value.DirRel)
	if len(files) == 0 {
		return
	}
	st := h.model.DedupView
	clear := false
	for _, f := range files {
		if st.Marked[f.Abs.String()] {
			clear = true
			break
		}
	}
	gi := mainSel.Value.GroupIdx
	size := int64(0)
	if gi >= 0 && gi < len(h.model.DedupSnapshot.Groups) {
		size = h.model.DedupSnapshot.Groups[gi].Size
	}
	for _, f := range files {
		abs := f.Abs.String()
		if !clear && st.Kept[abs] {
			continue
		}
		h.setMark(abs, size, !clear)
	}
}

func keepAbsSetFromFiles(files []comparepkg.DedupFile) map[string]bool {
	out := make(map[string]bool, len(files))
	for _, f := range files {
		out[f.Abs.String()] = true
	}
	return out
}

func dedupKeepSetsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

// applyGroupKeep designates keepAbs files as survivors in group gi and marks
// every other group member for deletion. When the group's current keep set
// already equals keepAbs, clears keep and deletion marks for the whole group.
// The second return value is true when an existing keep designation was replaced
// by a different keep set (not toggle-off).
func (h *Handler) applyGroupKeep(gi int, keepAbs map[string]bool) (applied bool, replacedKeep bool) {
	if gi < 0 || gi >= len(h.model.DedupSnapshot.Groups) || len(keepAbs) == 0 {
		return false, false
	}
	g := h.model.DedupSnapshot.Groups[gi]
	st := &h.model.DedupView
	if st.Kept == nil {
		st.Kept = map[string]bool{}
	}
	currentKeep := map[string]bool{}
	for _, f := range g.Files {
		if st.Kept[f.Abs.String()] {
			currentKeep[f.Abs.String()] = true
		}
	}
	if dedupKeepSetsEqual(currentKeep, keepAbs) {
		for _, f := range g.Files {
			abs := f.Abs.String()
			delete(st.Kept, abs)
			h.setMark(abs, g.Size, false)
		}
		return true, false
	}
	replacedKeep = len(currentKeep) > 0
	for _, f := range g.Files {
		abs := f.Abs.String()
		if keepAbs[abs] {
			st.Kept[abs] = true
			h.setMark(abs, g.Size, false)
		} else {
			delete(st.Kept, abs)
			h.setMark(abs, g.Size, true)
		}
	}
	return true, replacedKeep
}

func (h *Handler) notifyDuplicateKeep(replaced bool) {
	if replaced {
		h.host.SetTransientMessage("Duplicate keep", ui.MessageUrgencyInfo)
	}
}

func (h *Handler) keepCopyFilesUnderDir(dirRow ui.DedupRow) bool {
	mainSel, ok := h.paneRow(&h.model.DedupView.Main, h.model.DedupList)
	if !ok {
		return false
	}
	files := ui.DedupCopyFilesUnderDir(h.model.DedupSnapshot, mainSel, dirRow.Value.DirRel)
	if len(files) == 0 {
		return false
	}
	_, replaced := h.applyGroupKeep(mainSel.Value.GroupIdx, keepAbsSetFromFiles(files))
	return replaced
}

// KeepSelection marks the focused file or folder contents as survivors (green)
// and marks every other copy in the affected duplicate group(s) for deletion.
func (h *Handler) KeepSelection() {
	row, ok := h.selectedRow()
	if !ok {
		return
	}
	st := &h.model.DedupView
	if st.FocusCopies && row.Value.Kind == ui.DedupRowDir {
		h.notifyDuplicateKeep(h.keepCopyFilesUnderDir(row))
		_, rows := h.focusedPane()
		next := ui.DedupNextDirRowIndex(rows, st.Copies.Selected)
		if next >= len(rows) {
			next = len(rows) - 1
		}
		st.Copies.Selected = next
		return
	}
	if row.Value.Kind == ui.DedupRowDir && !st.FocusCopies {
		replaced := false
		byGroup := ui.DedupSnapshotFilesUnderDir(h.model.DedupSnapshot, row.Value.DirRel)
		for gi, files := range byGroup {
			if _, groupReplaced := h.applyGroupKeep(gi, keepAbsSetFromFiles(files)); groupReplaced {
				replaced = true
			}
		}
		h.notifyDuplicateKeep(replaced)
		return
	}
	if row.Value.Kind != ui.DedupRowFile {
		return
	}
	gi := row.Value.GroupIdx
	if st.FocusCopies {
		mainSel, ok := h.paneRow(&st.Main, h.model.DedupList)
		if !ok {
			return
		}
		gi = mainSel.Value.GroupIdx
	}
	_, replaced := h.applyGroupKeep(gi, map[string]bool{row.Value.AbsKey: true})
	h.notifyDuplicateKeep(replaced)
	h.MoveSelection(1)
}

// dirAbs returns the absolute directory of a directory row, or the parent of a file row.
func (h *Handler) dirAbs(row ui.DedupRow) string {
	if row.Value.Kind == ui.DedupRowDir {
		root := strings.TrimSuffix(h.model.DedupSnapshot.EffectiveDisplayRoot().String(), "/")
		return root + "/" + row.Value.DirRel
	}
	return row.Value.File.Abs.Parent().String()
}

// selectedDirAbs returns the absolute directory for navigation from the focused row.
func (h *Handler) selectedDirAbs() string {
	row, ok := h.selectedRow()
	if !ok {
		return ""
	}
	return h.dirAbs(row)
}

// OpenInPanel points the given panel at path (a directory, or a file's parent with the file
// selected) and leaves the view open. Single entry point for the tree panes and the browse panel.
func (h *Handler) OpenInPanel(panelID int, path string, isDir bool) {
	dir, name := path, ""
	if !isDir {
		dir, name = filepath.Dir(path), filepath.Base(path)
	}
	if err := h.host.NavigatePanelToPath(panelID, dir, name); err != nil {
		h.host.SetTransientMessage("Open: "+err.Error(), ui.MessageUrgencyWarn)
		return
	}
	which := "primary"
	if panelID == ui.SecondaryPanel {
		which = "secondary"
	}
	h.host.SetTransientMessage(fmt.Sprintf("Opened %s in %s panel", filepath.Base(path), which), ui.MessageUrgencyInfo)
}

// SelectedPinTarget is PaneTarget for the focused pane.
func (h *Handler) SelectedPinTarget() (path string, isDir bool, ok bool) {
	return h.PaneTarget(h.model.DedupView.FocusCopies)
}

// PaneTarget returns the absolute path and directory-ness of the named pane's cursor row
// (copies or main), for pinning and the browse panel. Dir rows give their directory, file rows
// their own path. False when the pane has no row or the row is neither a file nor a directory.
func (h *Handler) PaneTarget(copies bool) (path string, isDir bool, ok bool) {
	st := &h.model.DedupView
	pane, rows := &st.Main, h.model.DedupList
	if copies {
		pane, rows = &st.Copies, h.model.DedupCopiesList
	}
	row, rowOK := h.paneRow(pane, rows)
	if !rowOK {
		return "", false, false
	}
	switch row.Value.Kind {
	case ui.DedupRowDir:
		return h.dirAbs(row), true, true
	case ui.DedupRowFile:
		return row.Value.File.Abs.String(), false, true
	default:
		return "", false, false
	}
}

// PaneOpenableFile returns the focused-pane file row's path when it is a plain file; group
// headers (file rows with children) and directories report false.
func (h *Handler) PaneOpenableFile(copies bool) (string, bool) {
	st := &h.model.DedupView
	pane, rows := &st.Main, h.model.DedupList
	if copies {
		pane, rows = &st.Copies, h.model.DedupCopiesList
	}
	row, ok := h.paneRow(pane, rows)
	if !ok || row.HasChildren || row.Value.Kind != ui.DedupRowFile {
		return "", false
	}
	return row.Value.File.Abs.String(), true
}

// ClearMarks unmarks every file and clears keep designations, reusing the
// file-list clear-selection binding.
func (h *Handler) ClearMarks() {
	st := &h.model.DedupView
	if st.MarkedCount == 0 && len(st.Kept) == 0 {
		return
	}
	st.Marked = map[string]bool{}
	st.MarkedCount = 0
	st.MarkedReclaimBytes = 0
	st.Kept = map[string]bool{}
}

// MarkedFiles returns marked files that still exist in the current snapshot, in
// group order. Iterates the active groups (not visible rows) so files hidden
// inside collapsed nodes are included.
func (h *Handler) MarkedFiles() []comparepkg.DedupFile {
	st := h.model.DedupView
	out := make([]comparepkg.DedupFile, 0, len(st.Marked))
	for _, g := range h.activeGroups() {
		for _, f := range g.Files {
			if st.Marked[f.Abs.String()] {
				out = append(out, f)
			}
		}
	}
	return out
}

// MarkedPaths returns marked file paths that still exist in the current snapshot.
func (h *Handler) MarkedPaths() []string {
	files := h.MarkedFiles()
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Abs.String())
	}
	return out
}

// EmptyDirsAfterDelete previews the directories that deleting the currently
// marked files would leave empty (dry run, no filesystem changes), as paths
// relative to the dedup root. Returns nil when nothing would be removed, so
// callers can skip the "remove empty directories?" confirmation entirely.
func (h *Handler) EmptyDirsAfterDelete() []string {
	files := h.MarkedFiles()
	if len(files) == 0 {
		return nil
	}
	removed := make(map[string]bool, len(files))
	var roots []pathloc.Path
	seen := map[string]bool{}
	for _, f := range files {
		removed[f.Abs.String()] = true
		parent := f.Abs.Parent()
		if key := parent.String(); !seen[key] {
			seen[key] = true
			roots = append(roots, parent)
		}
	}
	dirs, err := ops.PreviewEmptyDirsUnder(context.Background(), roots, removed)
	if err != nil || len(dirs) == 0 {
		return nil
	}
	rootFP, err := h.model.DedupSnapshot.EffectiveDisplayRoot().FilePath()
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		dfp, err := d.FilePath()
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(rootFP, dfp)
		if err != nil {
			continue
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

// DeleteMarked enqueues a delete job for marked files and optimistically prunes them
// from the view.
func (h *Handler) DeleteMarked(removeEmptyDirs bool) {
	st := &h.model.DedupView
	paths := h.MarkedPaths()
	if len(paths) == 0 {
		return
	}
	h.host.EnqueueDeleteJob(paths, removeEmptyDirs)
	noun := "files"
	if len(paths) == 1 {
		noun = "file"
	}
	h.host.SetTransientMessage(fmt.Sprintf("Delete queued (%d %s)", len(paths), noun), ui.MessageUrgencyInfo)
	// Optimistically drop the deleted files; groups under two members disappear.
	// ponytail: no re-walk — if a delete fails the row just vanishes; reopen to rescan.
	oldRows, oldSel := h.model.DedupList, st.Main.Selected
	h.model.DedupSnapshot = h.model.DedupSnapshot.WithoutPaths(st.Marked)
	st.Marked = map[string]bool{}
	st.MarkedCount = 0
	st.MarkedReclaimBytes = 0
	st.Kept = map[string]bool{}
	st.FocusCopies = false
	h.syncDedupList()
	// Keep the cursor on the same row, else the nearest surviving one (next, then previous).
	st.Main.Selected = 0
	for _, i := range dedupNearestOrder(oldSel, len(oldRows)) {
		if j := ui.DedupRowIndexByID(h.model.DedupList, oldRows[i].ID); j >= 0 {
			st.Main.Selected = j
			break
		}
	}
	h.syncCopies()
	h.ensureSelectionVisible(0)
}

// dedupNearestOrder lists indexes [0,n) as sel, sel+1..n-1, then sel-1..0.
func dedupNearestOrder(sel, n int) []int {
	sel = min(max(sel, 0), n)
	out := make([]int, 0, n)
	for i := sel; i < n; i++ {
		out = append(out, i)
	}
	for i := sel - 1; i >= 0; i-- {
		out = append(out, i)
	}
	return out
}

// NavigateFromSelection opens the selected file's directory (or the selected
// dirs-mode directory itself) in the active panel.
func (h *Handler) NavigateFromSelection() {
	row, ok := h.selectedRow()
	if !ok {
		return
	}
	if row.Value.Kind == ui.DedupRowDir {
		_ = h.host.NavigatePanelToPath(h.model.ActivePanel, h.selectedDirAbs(), "")
		return
	}
	f := row.Value.File
	selectName := filepath.Base(filepath.FromSlash(f.Rel))
	_ = h.host.NavigatePanelToPath(h.model.ActivePanel, f.Abs.Parent().String(), selectName)
}

func (h *Handler) filterOptions() quickfilter.Options {
	f := h.host.Config().Filter
	return quickfilter.OptionsFrom(f.CaseInsensitive, f.CycleMatches)
}
