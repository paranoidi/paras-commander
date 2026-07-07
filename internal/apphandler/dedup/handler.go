// Package dedup owns the full-screen "find duplicates within a single directory" view.
package dedup

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/gitignore"
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
	NavigatePanelToPath(panelID int, path string, selectName string) error
	EnqueueDeleteJob(paths []string)
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	DedupMenuDefinitions() []menu.Definition
	BrowserMenuDefinitions() []menu.Definition
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

	session     *comparepkg.DedupSession
	wakePending atomic.Bool
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
	if h.screen == nil {
		return
	}
	if h.wakePending.Swap(true) {
		return
	}
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{}))
}

// Open starts scanning the active panel's directory for duplicates.
func (h *Handler) Open() {
	if ui.IsAuxiliaryView(h.model.ViewMode) && h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.Close()

	p := &h.model.Primary
	if h.model.ActivePanel == ui.SecondaryPanel {
		p = &h.model.Secondary
	}
	root := p.Path
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
	shouldSkip := diskusage.ComposeListingVolumeIgnore(h.diskIgnore, volGate)

	bufKiB := h.config.Compare.ReadBufferKiB
	if bufKiB <= 0 {
		bufKiB = config.DefaultCompareReadBufferKiB
	}
	workers := h.config.Compare.HashConcurrency
	if workers <= 0 {
		workers = config.DefaultCompareHashConcurrency
	}

	h.session = comparepkg.StartDedup(context.Background(), root, comparepkg.DedupOptions{
		Walk: comparepkg.WalkOptions{
			ShowHidden:    p.ShowHidden,
			Gitignore:     h.gitignore,
			ShouldSkipDir: shouldSkip,
		},
		HashWorkers:      workers,
		ReadBuffer:       make([]byte, bufKiB*1024),
		MaxHashBytes:     h.config.Compare.MaxHashBytes,
		ConfirmHashBytes: h.config.Dedup.HashConfirmBytes,
		OnUpdate:         func(_ comparepkg.DedupSnapshot) { h.postWake() },
	})
	h.model.DedupSnapshot = h.session.Snapshot()
}

// Close cancels the scan and returns to the browser.
func (h *Handler) Close() {
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
	h.model.DedupView = ui.DedupViewState{}
	h.model.DedupSnapshot = comparepkg.DedupSnapshot{}
	h.model.DedupList = nil
	h.model.DedupCopiesList = nil
}

// PollUpdates applies the latest session snapshot. Returns true when the UI should repaint.
func (h *Handler) PollUpdates(_ WakePayload) bool {
	h.wakePending.Store(false)
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
	h.model.DedupProgressDialog = dialog.DedupProgressDialogState{}
	h.model.ViewMode = ui.ViewDedup
	h.model.MenuDefinitions = h.host.DedupMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndexDedup()
	h.syncDedupList()
	h.ensureSelectionVisible(0)
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

// Refresh re-runs the scan on the active panel.
func (h *Handler) Refresh() {
	if h.model.ViewMode != ui.ViewDedup {
		return
	}
	h.Open()
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

// SwitchPane toggles focus between the main tree and the copies pane (Tab).
func (h *Handler) SwitchPane() {
	st := &h.model.DedupView
	if !st.FocusCopies && len(h.model.DedupCopiesList) == 0 {
		return
	}
	st.FocusCopies = !st.FocusCopies
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

// ToggleNode expands/collapses the focused pane's node under the cursor. On
// expand, a chain of single-subdirectory nodes is opened in one go (see
// autoDescendSingleDir).
func (h *Handler) ToggleNode() {
	row, ok := h.selectedRow()
	if !ok || !row.HasChildren {
		return
	}
	pane, _ := h.focusedPane()
	expanding := pane.Collapsed[row.ID]
	pane.SetCollapsed(row.ID, !expanding)
	h.resyncFocused()
	if expanding {
		h.autoDescendSingleDir()
	}
}

// autoDescendSingleDir walks down from the just-expanded node: while its only
// direct child is a directory, expand that child too and move the cursor onto
// it. All steps happen inside one key event, so only the end result is painted.
func (h *Handler) autoDescendSingleDir() {
	for {
		pane, rows := h.focusedPane()
		row, ok := h.paneRow(pane, rows)
		if !ok || !row.HasChildren || !row.Expanded {
			break
		}
		childIdx := -1
		children := 0
		for i := pane.Selected + 1; i < len(rows) && rows[i].Depth > row.Depth; i++ {
			if rows[i].Depth == row.Depth+1 {
				children++
				childIdx = i
			}
		}
		if children != 1 || rows[childIdx].Value.Kind != ui.DedupRowDir {
			break
		}
		child := rows[childIdx]
		pane.Selected = childIdx
		if pane.Collapsed[child.ID] {
			pane.SetCollapsed(child.ID, false)
			h.resyncFocused()
			pane, rows = h.focusedPane()
			i := ui.DedupRowIndexByID(rows, child.ID)
			if i < 0 {
				break
			}
			pane.Selected = i
		}
	}
	if !h.model.DedupView.FocusCopies {
		h.syncCopies()
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

// ExpandAll clears the focused pane's collapse state.
func (h *Handler) ExpandAll() {
	pane, _ := h.focusedPane()
	pane.Collapsed = nil
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

// ToggleMark flips the delete mark on the selected file. Directory rows are not
// markable (the mark-under actions cover whole directories).
func (h *Handler) ToggleMark() {
	row, ok := h.selectedRow()
	if !ok || row.Value.Kind != ui.DedupRowFile {
		return
	}
	st := h.model.DedupView
	h.setMark(row.Value.AbsKey, row.Value.Size, !st.Marked[row.Value.AbsKey])
}

// ToggleGroupMark marks every copy in the selected row's duplicate group, or unmarks
// them all when the whole group is already marked.
func (h *Handler) ToggleGroupMark() {
	row, ok := h.selectedRow()
	if !ok || row.Value.Kind != ui.DedupRowFile {
		return
	}
	snap := h.model.DedupSnapshot
	gi := row.Value.GroupIdx
	if gi < 0 || gi >= len(snap.Groups) {
		return
	}
	g := snap.Groups[gi]
	unmark := ui.DedupGroupFullyMarked(g, h.model.DedupView.Marked)
	for _, f := range g.Files {
		h.setMark(f.Abs.String(), g.Size, !unmark)
	}
}

// MarkRedundantUnderSelected marks (for deletion) redundant duplicate copies under
// the selected row's directory, leaving one surviving copy of each content group so
// only unique files remain there ("keep uniques"). Mark-only — deletion stays with
// DeleteMarked.
func (h *Handler) MarkRedundantUnderSelected() {
	h.markUnderSelected(ui.DedupRedundantUnder, "No redundant copies under this folder")
}

// MarkDuplicatesUnderSelected marks (for deletion) copies under the selected row's
// directory that are also stored outside it, leaving groups that live only here
// untouched ("delete duplicates from here"). Mark-only — deletion stays with
// DeleteMarked.
func (h *Handler) MarkDuplicatesUnderSelected() {
	h.markUnderSelected(ui.DedupDuplicatesUnder, "No duplicates stored elsewhere under this folder")
}

// selectedDirAbs returns the absolute directory the mark-under actions operate
// on: the directory itself when the cursor is on a dirs-mode directory row,
// otherwise the selected file's parent directory.
func (h *Handler) selectedDirAbs() string {
	row, ok := h.selectedRow()
	if !ok {
		return ""
	}
	if row.Value.Kind == ui.DedupRowDir {
		root := strings.TrimSuffix(h.model.DedupSnapshot.Root.String(), "/")
		return root + "/" + row.Value.DirRel
	}
	return row.Value.File.Abs.Parent().String()
}

// markUnderSelected applies a mark-selection rule over the active groups and
// merges the result into the delete-mark set. Operating on groups (not visible
// rows) keeps collapsed copies included.
func (h *Handler) markUnderSelected(pick func([]comparepkg.DedupGroup, string) []string, emptyMsg string) {
	groups := h.activeGroups()
	keys := pick(groups, h.selectedDirAbs())
	if len(keys) == 0 {
		h.host.SetTransientMessage(emptyMsg, ui.MessageUrgencyInfo)
		return
	}
	sizeByKey := make(map[string]int64, len(keys))
	for _, g := range groups {
		for _, f := range g.Files {
			sizeByKey[f.Abs.String()] = g.Size
		}
	}
	for _, k := range keys {
		h.setMark(k, sizeByKey[k], true)
	}
}

// ClearMarks unmarks every file, reusing the file-list clear-selection binding.
func (h *Handler) ClearMarks() {
	st := &h.model.DedupView
	if st.MarkedCount == 0 {
		return
	}
	st.Marked = map[string]bool{}
	st.MarkedCount = 0
	st.MarkedReclaimBytes = 0
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

// DeleteMarked enqueues a delete job for marked files and optimistically prunes them
// from the view.
func (h *Handler) DeleteMarked() {
	st := &h.model.DedupView
	paths := h.MarkedPaths()
	if len(paths) == 0 {
		return
	}
	h.host.EnqueueDeleteJob(paths)
	noun := "files"
	if len(paths) == 1 {
		noun = "file"
	}
	h.host.SetTransientMessage(fmt.Sprintf("Delete queued (%d %s)", len(paths), noun), ui.MessageUrgencyInfo)
	// Optimistically drop the deleted files; groups under two members disappear.
	// ponytail: no re-walk — if a delete fails the row just vanishes; reopen to rescan.
	h.model.DedupSnapshot = h.model.DedupSnapshot.WithoutPaths(st.Marked)
	st.Marked = map[string]bool{}
	st.MarkedCount = 0
	st.MarkedReclaimBytes = 0
	st.Main = ui.DedupPane{Collapsed: st.Main.Collapsed}
	st.FocusCopies = false
	h.syncDedupList()
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
