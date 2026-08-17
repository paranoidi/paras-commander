package find

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/scan"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (h *Handler) OpenDialog(panelID int) {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	if h.host.InQuickFilterUI() {
		h.host.ActivePanel().CancelFilter(h.host.ActiveViewportRows())
	}
	if h.model.FindDialog.Open {
		h.CloseDialog()
	}
	p := h.host.PanelByID(panelID)
	root := filepath.Clean(p.PathString())
	selRoots := panel.PruneNestedPaths(p.SelectedDirectoryPaths())
	h.model.FindDialog = dialog.FindDialogState{
		Open:                       true,
		PanelID:                    panelID,
		RootPath:                   root,
		IncludeHidden:              false,
		StayOnCurrentVolume:        true,
		ListingDevice:              p.ListingDevice,
		ListingDeviceValid:         p.ListingDeviceValid,
		ShowSearchSelectionsOption: len(selRoots) > 0,
		SearchOnlySelections:       len(selRoots) > 0,
		SelectionDirRoots:          selRoots,
		Focus:                      0,
		Indexing:                   true,
	}
	h.bindFindDialogPathMeta(&h.model.FindDialog)
	h.startFindIndexer()
}

func (h *Handler) CloseDialog() {
	h.stopFindIndexer()
	h.cancelPendingRank()
	h.model.FindDialog = dialog.FindDialogState{}
}

func (h *Handler) findVolumeGate(st *dialog.FindDialogState) diskusage.ListingVolumeGate {
	return diskusage.ListingVolumeGate{
		Enabled: st.StayOnCurrentVolume && st.ListingDeviceValid,
		RefDev:  st.ListingDevice,
		Valid:   st.ListingDeviceValid,
	}
}

func (h *Handler) findScopeRoots(st *dialog.FindDialogState) []string {
	if st.ShowSearchSelectionsOption && st.SearchOnlySelections && len(st.SelectionDirRoots) > 0 {
		out := make([]string, len(st.SelectionDirRoots))
		for i, r := range st.SelectionDirRoots {
			out[i] = filepath.Clean(r)
		}
		return out
	}
	return []string{filepath.Clean(st.RootPath)}
}

// findIndexingSkipsMatch is true while a walk is running with an empty query.
// Display rank still updates incrementally; only background fuzzy match is skipped.
func findIndexingSkipsMatch(st *dialog.FindDialogState) bool {
	return st.Indexing && search.Parse(st.Query).Empty()
}

// findIndexingSkipsRank is an alias kept for call sites; see findIndexingSkipsMatch.
func findIndexingSkipsRank(st *dialog.FindDialogState) bool {
	return findIndexingSkipsMatch(st)
}

func findIndexingCountThrottle(indexed int) time.Duration {
	ms := config.DefaultFindIndexingCountThrottleMS
	switch {
	case indexed >= findIndexLargeThreshold:
		ms *= 2
	case indexed >= findIndexMediumThreshold:
		ms += ms / 2
	}
	return time.Duration(ms) * time.Millisecond
}

// maybeRenderFindIndexing reports whether the find dialog title/count should repaint now.
// While indexing, repaints are throttled; non-indexing callers always get true.
func (h *Handler) maybeRenderFindIndexing(st *dialog.FindDialogState) bool {
	if !st.Open {
		return false
	}
	if !st.Indexing {
		return true
	}
	interval := findIndexingCountThrottle(st.IndexedCount)
	if time.Since(h.lastIndexCountRenderAt) >= interval {
		h.lastIndexCountRenderAt = time.Now()
		return true
	}
	return false
}

func (h *Handler) applyEmptyQueryDisplayRank(st *dialog.FindDialogState) {
	maxResults := h.config.UI.Find.MaxResults
	st.Ranked = emptyQueryDisplayIndicesFromEntries(st.Entries, st.OnlyDirectories, st.OnlyFiles, maxResults)
	st.RankDisplayLines = nil
	st.MatchRanges = nil
	st.FullRanked = nil
	st.FullRankedEntriesLen = len(st.Entries)
	st.FullRankedOnlyDirs = st.OnlyDirectories
	st.FullRankedOnlyFiles = st.OnlyFiles
	st.RankPending = false
	if st.Selected >= len(st.Ranked) {
		if len(st.Ranked) == 0 {
			st.Selected = 0
		} else {
			st.Selected = len(st.Ranked) - 1
		}
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	dialog.EnsureFindListScroll(st, h.findDialogListRows())
}

// extendEmptyQueryDisplayRank appends walk-order indices for newly mirrored entries.
func (h *Handler) extendEmptyQueryDisplayRank(st *dialog.FindDialogState, fromIdx int) {
	prevLen := len(st.Ranked)
	st.Ranked = appendEmptyQueryDisplayIndices(
		st.Ranked, st.Entries, fromIdx,
		st.OnlyDirectories, st.OnlyFiles, h.config.UI.Find.MaxResults,
	)
	if len(st.Ranked) == prevLen {
		return
	}
	st.RankDisplayLines = nil
	st.MatchRanges = nil
	st.RankPending = false
	if st.Selected >= len(st.Ranked) {
		if len(st.Ranked) == 0 {
			st.Selected = 0
		} else {
			st.Selected = len(st.Ranked) - 1
		}
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	dialog.EnsureFindListScroll(st, h.findDialogListRows())
}

func appendEmptyQueryDisplayIndices(
	ranked []int,
	entries []dialog.FindEntry,
	fromIdx int,
	onlyDirs, onlyFiles bool,
	maxResults int,
) []int {
	if fromIdx < 0 {
		fromIdx = 0
	}
	if fromIdx >= len(entries) {
		return ranked
	}
	cap := maxResults
	if cap <= 0 {
		cap = len(entries)
	}
	if len(ranked) >= cap {
		return ranked
	}
	for i := fromIdx; i < len(entries); i++ {
		ent := entries[i]
		if onlyDirs && !ent.IsDir {
			continue
		}
		if onlyFiles && ent.IsDir {
			continue
		}
		ranked = append(ranked, i)
		if len(ranked) >= cap {
			break
		}
	}
	return ranked
}

func emptyQueryDisplayIndicesFromEntries(entries []dialog.FindEntry, onlyDirs, onlyFiles bool, maxResults int) []int {
	if len(entries) == 0 {
		return nil
	}
	cap := maxResults
	if cap <= 0 {
		cap = len(entries)
	}
	if !onlyDirs && !onlyFiles {
		n := len(entries)
		if maxResults > 0 && n > maxResults {
			n = maxResults
		}
		out := make([]int, n)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, cap)
	for i := range entries {
		if onlyDirs && !entries[i].IsDir {
			continue
		}
		if onlyFiles && entries[i].IsDir {
			continue
		}
		out = append(out, i)
		if len(out) >= cap {
			break
		}
	}
	return out
}

func emptyQueryDisplayIndices(n int, onlyDirs, onlyFiles bool, isDirs []bool, maxResults int) []int {
	if n == 0 {
		return nil
	}
	cap := n
	if maxResults > 0 && cap > maxResults {
		cap = maxResults
	}
	if !onlyDirs && !onlyFiles {
		out := make([]int, cap)
		for i := range out {
			out[i] = i
		}
		return out
	}
	out := make([]int, 0, cap)
	for i := 0; i < n && len(out) < cap; i++ {
		if onlyDirs && (i >= len(isDirs) || !isDirs[i]) {
			continue
		}
		if onlyFiles && (i >= len(isDirs) || isDirs[i]) {
			continue
		}
		out = append(out, i)
	}
	return out
}

func findEntryAbsPath(st *dialog.FindDialogState, ent dialog.FindEntry) string {
	return filepath.Clean(ent.AbsPath(st.RootPath))
}

func (h *Handler) findPathIsDir(st *dialog.FindDialogState) func(string) bool {
	return func(path string) bool {
		if st.PathMeta == nil {
			return false
		}
		isDir, _, ok := st.PathMeta(filepath.Clean(path))
		return ok && isDir
	}
}

func (h *Handler) narrowFindIndexer() {
	st := &h.model.FindDialog
	st.IndexDone = false
	st.Indexing = true
	h.scan.NarrowSelection(st.SelectionDirRoots)
}

func (h *Handler) widenFindIndexer() {
	st := &h.model.FindDialog
	st.IndexDone = false
	st.Indexing = true
	h.scan.Widen()
}

// ToggleSearchOnlySelections toggles search-only-selections mode.
func (h *Handler) ToggleSearchOnlySelections() {
	st := &h.model.FindDialog
	if !st.ShowSearchSelectionsOption {
		return
	}
	st.SearchOnlySelections = !st.SearchOnlySelections
	if st.SearchOnlySelections {
		h.narrowFindIndexer()
	} else {
		h.widenFindIndexer()
	}
}

func (h *Handler) syncFindDialogRanks() {
	st := &h.model.FindDialog
	if !st.Open || findIndexingSkipsRank(st) {
		return
	}
	if search.Parse(st.Query).Empty() {
		h.applyEmptyQueryDisplayRank(st)
		return
	}
	h.scheduleFindRank(0)
}

// cancelPendingRank increments the generation counter so any in-flight or scheduled rank is discarded.
func (h *Handler) cancelPendingRank() {
	h.rankMu.Lock()
	h.rankGen++
	if h.rankTimer != nil {
		h.rankTimer.Stop()
		h.rankTimer = nil
	}
	h.rankMu.Unlock()
}

func (h *Handler) dispatchMatch(gen int) {
	st := &h.model.FindDialog
	h.scan.RequestMatch(scan.MatchRequest{
		Gen:             gen,
		Query:           st.Query,
		OnlyDirs:        st.OnlyDirectories,
		OnlyFiles:       st.OnlyFiles,
		CaseInsensitive: h.config.Filter.CaseInsensitive,
		MaxResults:      h.config.UI.Find.MaxResults,
	})
}

// scheduleFindRank schedules a background match on the scan coordinator.
func (h *Handler) scheduleFindRank(ms int) {
	st := &h.model.FindDialog
	if !st.Open {
		return
	}
	if findIndexingSkipsRank(st) {
		st.RankPending = false
		return
	}

	st.RankPending = true

	if ms == 0 {
		// Immediate: cancel any pending timer/wake, increment gen, send now.
		// Also clears nav-idle debounce so the result is applied without delay.
		h.clearFindNavIdle()
		h.rankMu.Lock()
		if h.rankTimer != nil {
			h.rankTimer.Stop()
			h.rankTimer = nil
		}
		h.throttleWakePending = false
		h.debouncePending = false
		h.rankGen++
		gen := h.rankGen
		h.lastRankSentAt = time.Now()
		h.rankMu.Unlock()
		h.dispatchMatch(gen)
		return
	}

	if ms < 0 {
		// Debounce: reset timer on every call. When the timer fires it posts
		// DebounceRankWakePayload, which increments gen to cancel any in-flight
		// computation for the previous query before dispatching fresh work.
		delay := time.Duration(-ms) * time.Millisecond
		h.rankMu.Lock()
		if h.rankTimer != nil {
			h.rankTimer.Stop()
		}
		h.rankTimer = time.AfterFunc(delay, func() {
			h.rankMu.Lock()
			h.rankTimer = nil
			h.debouncePending = true
			h.rankMu.Unlock()
			_ = h.screen.PostEvent(tcell.NewEventInterrupt(DebounceRankWakePayload{}))
		})
		h.rankMu.Unlock()
		return
	}

	// Throttle (ms > 0): fire immediately if the cooldown has elapsed; otherwise ensure a
	// single trailing-edge timer is running. Gen is NOT incremented — indexing batches only
	// bring fresher data for the same query; only query changes (debounce) need gen bumps.
	interval := time.Duration(ms) * time.Millisecond
	h.rankMu.Lock()
	elapsed := time.Since(h.lastRankSentAt)
	if elapsed >= interval {
		if h.rankTimer != nil {
			h.rankTimer.Stop()
			h.rankTimer = nil
		}
		h.throttleWakePending = false
		gen := h.rankGen // do NOT increment; let in-flight work for the current query finish
		h.lastRankSentAt = time.Now()
		h.rankMu.Unlock()
		h.dispatchMatch(gen)
		return
	}
	// Within cooldown: start a trailing-edge timer once (never reset).
	if h.rankTimer == nil && !h.throttleWakePending {
		remaining := interval - elapsed
		h.rankTimer = time.AfterFunc(remaining, func() {
			h.rankMu.Lock()
			h.rankTimer = nil
			h.throttleWakePending = true
			h.rankMu.Unlock()
			_ = h.screen.PostEvent(tcell.NewEventInterrupt(ThrottleRankWakePayload{}))
		})
	}
	h.rankMu.Unlock()
}

// HandleThrottleRankWake is called by the main thread when a ThrottleRankWakePayload event
// arrives (indexing-batch trailing-edge timer). It takes a fresh snapshot and sends it to
// the rank worker WITHOUT incrementing rankGen, so any in-flight computation is allowed to
// finish and deliver its partial result before the worker picks up the fresher snapshot.
// Returns true when the dialog is open, a wake was pending, and a rank was scheduled.
func (h *Handler) HandleThrottleRankWake() bool {
	st := &h.model.FindDialog
	if !st.Open || findIndexingSkipsRank(st) {
		return false
	}
	h.rankMu.Lock()
	if !h.throttleWakePending {
		h.rankMu.Unlock()
		return false
	}
	h.throttleWakePending = false
	gen := h.rankGen // do NOT increment: let in-flight work deliver its partial result
	h.lastRankSentAt = time.Now()
	h.rankMu.Unlock()
	st.RankPending = true
	h.dispatchMatch(gen)
	return true
}

// HandleDebounceRankWake is called by the main thread when a DebounceRankWakePayload event
// arrives (query-typing debounce timer). It increments rankGen to cancel any in-flight rank
// for the old query, then dispatches fresh work for the new query.
// Returns true when the dialog is open, a wake was pending, and a rank was scheduled.
func (h *Handler) HandleDebounceRankWake() bool {
	st := &h.model.FindDialog
	if !st.Open {
		return false
	}
	h.rankMu.Lock()
	if !h.debouncePending {
		h.rankMu.Unlock()
		return false
	}
	h.debouncePending = false
	h.rankGen++ // cancel in-flight work for the previous query
	gen := h.rankGen
	h.lastRankSentAt = time.Now()
	h.rankMu.Unlock()
	// A query change implies the user wants fresh results — bypass nav-idle debounce.
	h.clearFindNavIdle()
	st.RankPending = true
	h.dispatchMatch(gen)
	return true
}

// armFindNavIdleTimer resets the navigation-idle debounce timer.
func (h *Handler) armFindNavIdleTimer() {
	if h.findNavTimer != nil {
		h.findNavTimer.Stop()
		h.findNavTimer = nil
	}
	idleMS := h.config.UI.Find.ListNavIdleMS
	if idleMS <= 0 {
		h.findNavActive = false
		return
	}
	h.findNavEpoch++
	h.findNavActive = true
	epochSnap := h.findNavEpoch
	delay := time.Duration(idleMS) * time.Millisecond
	h.findNavTimer = time.AfterFunc(delay, func() {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(FindNavIdlePayload{Epoch: epochSnap}))
	})
}

// clearFindNavIdle cancels any pending nav-idle timer and clears the active flag.
// Call this whenever rank results must be applied immediately (query change, option toggle).
// Called only from the main thread.
func (h *Handler) clearFindNavIdle() {
	if h.findNavTimer != nil {
		h.findNavTimer.Stop()
		h.findNavTimer = nil
	}
	h.findNavActive = false
	h.findNavEpoch++
}

// HandleFindNavIdle is called by the main thread when a FindNavIdlePayload event arrives.
// It clears the nav-active flag and applies any deferred rank result.
// Returns true if a result was applied and the UI needs a re-render.
func (h *Handler) HandleFindNavIdle(epoch uint64) bool {
	st := &h.model.FindDialog
	if !st.Open {
		return false
	}
	if epoch != h.findNavEpoch {
		return false // stale timer from before a query change or explicit clear
	}
	if h.findNavTimer != nil {
		h.findNavTimer.Stop()
		h.findNavTimer = nil
	}
	h.findNavActive = false
	return h.ApplyPendingRank()
}

// ApplyPendingRank applies the latest completed rank result (if any) to the dialog state.
func (h *Handler) ApplyPendingRank() bool {
	st := &h.model.FindDialog
	if !st.Open {
		return false
	}
	if h.findNavActive {
		return false // nav-idle debounce: wait until user stops scrolling
	}
	h.rankMu.Lock()
	gen := h.rankGen
	result := h.pendingRank
	h.pendingRank = nil
	h.rankMu.Unlock()

	if result == nil || result.gen != gen {
		return false
	}

	var selectedRelLine string
	if st.Selected >= 0 && st.Selected < len(st.Ranked) {
		if st.Selected < len(st.RankDisplayLines) {
			selectedRelLine = st.RankDisplayLines[st.Selected]
		} else if entIdx := st.Ranked[st.Selected]; entIdx >= 0 {
			if ent, ok := st.FindEntryAt(entIdx); ok {
				selectedRelLine = ent.RelLine
			}
		}
	}

	st.Ranked = result.ranked
	st.RankDisplayLines = result.rankDisplayLines
	st.MatchRanges = result.matchRanges
	st.FullRanked = result.fullRanked
	st.FullRankedGen = gen
	st.FullRankedEntriesLen = result.entriesLen
	st.FullRankedOnlyDirs = result.onlyDirs
	st.FullRankedOnlyFiles = result.onlyFiles

	if selectedRelLine != "" {
		for i, entIdx := range st.Ranked {
			line := ""
			if i < len(st.RankDisplayLines) {
				line = st.RankDisplayLines[i]
			} else if ent, ok := st.FindEntryAt(entIdx); ok {
				line = ent.RelLine
			}
			if line == selectedRelLine {
				st.Selected = i
				break
			}
		}
	}
	if st.Selected >= len(st.Ranked) {
		if len(st.Ranked) == 0 {
			st.Selected = 0
		} else {
			st.Selected = len(st.Ranked) - 1
		}
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
	// Center the selected item so updates don't jump the view.
	dialog.CenterFindListScroll(st, h.findDialogListRows())
	st.RankPending = false
	return true
}

func (h *Handler) findDialogListRows() int {
	termW, termH := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(termW, termH)
	return ui.FindDialogListRows(layout, h.model.FindDialog.ShowSearchSelectionsOption)
}

// ActivateDialogOK applies the find dialog OK action (navigate / apply marks).
func (h *Handler) ActivateDialogOK() {
	if h.findDialogMarkedCount() > 0 {
		h.applyFindDialogMarkedSelections()
		return
	}
	h.NavigateFindCursor()
}

func (h *Handler) findDialogMarkedCount() int {
	st := &h.model.FindDialog
	if len(st.MarkedPaths) == 0 {
		return 0
	}
	n := 0
	for path, on := range st.MarkedPaths {
		if on && path != "" {
			n++
		}
	}
	return n
}

func (h *Handler) applyFindDialogMarkedSelections() {
	st := &h.model.FindDialog
	p := h.host.PanelByID(st.PanelID)
	paths := make([]string, 0, len(st.MarkedPaths))
	for path, on := range st.MarkedPaths {
		if !on {
			continue
		}
		path = filepath.Clean(path)
		if path == "" {
			continue
		}
		if p.SelectedPaths != nil && p.SelectedPaths[path] {
			continue
		}
		paths = append(paths, path)
	}
	added := len(paths)
	conflicts := false
	if added > 0 {
		isDir := h.findPathIsDir(st)
		conflicts = p.BulkAddSelections(paths, isDir)
	}
	h.model.ActivePanel = st.PanelID
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseDialog()
	if added == 0 {
		h.host.SetTransientMessage("No new selections", ui.MessageUrgencyInfo)
		return
	}
	if conflicts {
		h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		return
	}
	h.host.SetTransientMessage(fmt.Sprintf("Added %d to selection", added), ui.MessageUrgencyInfo)
}

// navigateFindEntryToPanel points panelID at the currently selected result
// (cd into a directory; cd into a file's parent and highlight it). It does not
// change the active panel or close the dialog. Returns the entry basename and
// true on success; false if there is no valid selection or navigation fails.
func (h *Handler) navigateFindEntryToPanel(panelID int) (string, bool) {
	st := &h.model.FindDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return "", false
	}
	entIdx := st.Ranked[st.Selected]
	ent, ok := st.FindEntryAt(entIdx)
	if !ok {
		return "", false
	}
	path := findEntryAbsPath(st, ent)

	dir, name := path, ""
	if !ent.IsDir {
		dir = filepath.Clean(filepath.Dir(path))
		name = filepath.Base(path)
	}
	if err := h.host.NavigatePanelToPath(panelID, dir, name); err != nil {
		h.host.SetErrorMessage("Find", err)
		return "", false
	}
	h.host.PanelByID(panelID).EnsureCursorVisible(h.host.PanelViewportRows(panelID))
	return filepath.Base(path), true
}

// NavigateFindCursor moves the active panel to the find dialog selection and
// closes the dialog.
func (h *Handler) NavigateFindCursor() {
	panelID := h.model.FindDialog.PanelID
	if _, ok := h.navigateFindEntryToPanel(panelID); !ok {
		return
	}
	h.model.ActivePanel = panelID
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseDialog()
}

// OpenSelectedInPrimary points the primary (left) panel at the selected result,
// leaving the find dialog open.
func (h *Handler) OpenSelectedInPrimary() {
	if name, ok := h.navigateFindEntryToPanel(ui.PrimaryPanel); ok {
		h.host.SetTransientMessage(fmt.Sprintf("Opened %s in primary panel", name), ui.MessageUrgencyInfo)
	}
}

// OpenSelectedInSecondary points the secondary (right) panel at the selected
// result, leaving the find dialog open.
func (h *Handler) OpenSelectedInSecondary() {
	if name, ok := h.navigateFindEntryToPanel(ui.SecondaryPanel); ok {
		h.host.SetTransientMessage(fmt.Sprintf("Opened %s in secondary panel", name), ui.MessageUrgencyInfo)
	}
}

// OpenSelectedFullscreenPreview opens the highlighted find result in the
// fullscreen file viewer (F3) and closes the find dialog.
func (h *Handler) OpenSelectedFullscreenPreview() {
	st := &h.model.FindDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	ent, ok := st.FindEntryAt(st.Ranked[st.Selected])
	if !ok {
		return
	}
	path := findEntryAbsPath(st, ent)
	if ent.IsDir {
		h.host.SetTransientMessage("View: not a file", ui.MessageUrgencyWarn)
		return
	}
	err := localfs.CheckFilePreviewable(path)
	isImage := errors.Is(err, localfs.ErrFilePreviewImage)
	isMedia := errors.Is(err, localfs.ErrFilePreviewMedia)
	if err != nil && !isImage && !isMedia {
		if errors.Is(err, localfs.ErrFilePreviewBinary) {
			h.host.SetTransientMessage("View: not a text file", ui.MessageUrgencyWarn)
		} else {
			h.host.SetErrorMessage("View", err)
		}
		return
	}
	h.CloseDialog()
	if err := h.host.OpenFullscreenFilePreviewAt(path); err != nil {
		h.host.SetTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
	}
}

func (h *Handler) findDialogResultIndices(st *dialog.FindDialogState) []int {
	q := search.Parse(st.Query)
	if q.Empty() {
		return nil
	}
	h.rankMu.Lock()
	gen := h.rankGen
	h.rankMu.Unlock()
	if st.FullRankedGen == gen &&
		st.FullRankedEntriesLen == len(st.Entries) &&
		st.FullRankedOnlyDirs == st.OnlyDirectories &&
		st.FullRankedOnlyFiles == st.OnlyFiles &&
		len(st.FullRanked) > 0 {
		return st.FullRanked
	}
	return h.rankFindCorpusIndices(st)
}

func (h *Handler) rankFindCorpusIndices(st *dialog.FindDialogState) []int {
	if len(st.Entries) == 0 {
		return nil
	}
	ranked, _ := dialog.RankFindEntries(
		st.Entries,
		st.Query,
		st.OnlyDirectories,
		st.OnlyFiles,
		h.config.Filter.CaseInsensitive,
	)
	return ranked
}

func (h *Handler) findDialogSelectAll() {
	st := &h.model.FindDialog
	if search.Parse(st.Query).Empty() {
		h.findDialogSelectAllEmptyQuery(st)
		return
	}
	indices := h.findDialogResultIndices(st)
	if len(indices) == 0 {
		return
	}
	h.findDialogSelectAllIndices(st, indices)
}

func (h *Handler) findDialogSelectAllEmptyQuery(st *dialog.FindDialogState) {
	if len(st.Entries) == 0 {
		return
	}
	walkOrder := len(st.MarkedPaths) == 0
	if st.MarkedPaths == nil {
		st.MarkedPaths = make(map[string]bool)
	}
	paths := make([]string, 0, 256)
	for _, ent := range st.Entries {
		if st.OnlyDirectories && !ent.IsDir {
			continue
		}
		if st.OnlyFiles && ent.IsDir {
			continue
		}
		path := findEntryAbsPath(st, ent)
		if path == "" || st.MarkedPaths[path] {
			continue
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return
	}
	isDir := h.findPathIsDir(st)
	var conflicts bool
	if walkOrder {
		conflicts = panel.BulkApplySelectionAddsWalkOrder(st.MarkedPaths, paths, isDir)
	} else {
		conflicts = panel.BulkApplySelectionAdds(st.MarkedPaths, paths, isDir)
	}
	if conflicts {
		h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
	}
	st.InvalidateMarkedSelectionDerived()
}

func (h *Handler) findDialogSelectAllIndices(st *dialog.FindDialogState, indices []int) {
	walkOrder := len(st.MarkedPaths) == 0
	if st.MarkedPaths == nil {
		st.MarkedPaths = make(map[string]bool, len(indices))
	}
	paths := make([]string, 0, len(indices))
	for _, entIdx := range indices {
		ent, ok := st.FindEntryAt(entIdx)
		if !ok {
			continue
		}
		path := findEntryAbsPath(st, ent)
		if path == "" || st.MarkedPaths[path] {
			continue
		}
		paths = append(paths, path)
	}
	if len(paths) == 0 {
		return
	}
	isDir := h.findPathIsDir(st)
	var conflicts bool
	if walkOrder {
		conflicts = panel.BulkApplySelectionAddsWalkOrder(st.MarkedPaths, paths, isDir)
	} else {
		conflicts = panel.BulkApplySelectionAdds(st.MarkedPaths, paths, isDir)
	}
	if conflicts {
		h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
	}
	st.InvalidateMarkedSelectionDerived()
}

func (h *Handler) findDialogUnselectAll() {
	st := &h.model.FindDialog
	if len(st.MarkedPaths) == 0 {
		return
	}
	st.MarkedPaths = nil
	st.InvalidateMarkedSelectionDerived()
}

func (h *Handler) findDialogToggleSelectionAndAdvance() {
	st := &h.model.FindDialog
	if st.Focus != 0 || len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	ent, ok := st.FindEntryAt(entIdx)
	if !ok {
		return
	}
	path := findEntryAbsPath(st, ent)
	if path == "" {
		return
	}
	if st.MarkedPaths == nil {
		st.MarkedPaths = make(map[string]bool)
	}
	if st.MarkedPaths[path] {
		delete(st.MarkedPaths, path)
	} else {
		if panel.ApplySelectionAdds(st.MarkedPaths, []string{path}, h.findPathIsDir(st)) {
			h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
	}
	if st.Selected < len(st.Ranked)-1 {
		st.Selected++
		dialog.EnsureFindListScroll(st, h.findDialogListRows())
	}
	st.InvalidateMarkedSelectionDerived()
}

// ToggleStayOnVolume toggles stay-on-volume and restarts indexing when needed.
func (h *Handler) ToggleStayOnVolume() {
	st := &h.model.FindDialog
	st.StayOnCurrentVolume = !st.StayOnCurrentVolume
	h.restartFindIndexer()
}

// ToggleIncludeHidden toggles include-hidden via the scan coordinator.
func (h *Handler) ToggleIncludeHidden() {
	st := &h.model.FindDialog
	st.IncludeHidden = !st.IncludeHidden
	if st.IncludeHidden {
		st.IndexDone = false
		st.Indexing = true
	}
	h.scan.SetIncludeHidden(st.IncludeHidden)
	if st.IncludeHidden {
		h.clearFindNavIdle()
		if search.Parse(st.Query).Empty() && !st.Indexing {
			h.applyEmptyQueryDisplayRank(st)
		} else if !findIndexingSkipsRank(st) {
			h.scheduleFindRank(0)
		}
		return
	}
	// Strip runs async; PollUpdates applies IndexReplaced when done.
	st.IndexDone = false
	st.Indexing = true
	st.RankPending = !search.Parse(st.Query).Empty()
}

// ToggleOnlyDirectories toggles the only-directories result filter.
func (h *Handler) ToggleOnlyDirectories() {
	st := &h.model.FindDialog
	st.OnlyDirectories = !st.OnlyDirectories
	if st.OnlyDirectories {
		st.OnlyFiles = false
	}
	h.clearFindNavIdle()
	h.syncFindDialogRanks()
}

// ToggleOnlyFiles toggles the only-files result filter.
func (h *Handler) ToggleOnlyFiles() {
	st := &h.model.FindDialog
	st.OnlyFiles = !st.OnlyFiles
	if st.OnlyFiles {
		st.OnlyDirectories = false
	}
	h.clearFindNavIdle()
	h.syncFindDialogRanks()
}

// findToggleField identifies one of the toggleable find-dialog checkbox rows,
// shared by the Enter switch, the Alt-letter switch, and the plain Rune/space
// switch in HandleDialogKey so the focus/rune → Toggle* mapping lives once.
type findToggleField int

const (
	findToggleNone findToggleField = iota
	findToggleStayOnVolume
	findToggleIncludeHidden
	findToggleOnlyDirs
	findToggleOnlyFiles
	findToggleSelections
)

// findToggleFieldForRune maps a mnemonic letter (v/i/d/l/s) to its toggle field.
func findToggleFieldForRune(r rune) findToggleField {
	switch r {
	case 'v', 'V':
		return findToggleStayOnVolume
	case 'i', 'I':
		return findToggleIncludeHidden
	case 'd', 'D':
		return findToggleOnlyDirs
	case 'l', 'L':
		return findToggleOnlyFiles
	case 's', 'S':
		return findToggleSelections
	default:
		return findToggleNone
	}
}

// findToggleFieldForFocus maps a dialog focus index to its toggle field.
func (h *Handler) findToggleFieldForFocus(focus int) findToggleField {
	st := &h.model.FindDialog
	switch focus {
	case st.FindDialogStayOnVolumeFocus():
		return findToggleStayOnVolume
	case st.FindDialogIncludeHiddenFocus():
		return findToggleIncludeHidden
	case st.FindDialogOnlyDirsFocus():
		return findToggleOnlyDirs
	case st.FindDialogOnlyFilesFocus():
		return findToggleOnlyFiles
	case st.FindDialogSelectionsFocus():
		return findToggleSelections
	default:
		return findToggleNone
	}
}

// applyFindToggle runs the Toggle* method for field. Returns false (no-op) for findToggleNone.
func (h *Handler) applyFindToggle(field findToggleField) bool {
	switch field {
	case findToggleStayOnVolume:
		h.ToggleStayOnVolume()
	case findToggleIncludeHidden:
		h.ToggleIncludeHidden()
	case findToggleOnlyDirs:
		h.ToggleOnlyDirectories()
	case findToggleOnlyFiles:
		h.ToggleOnlyFiles()
	case findToggleSelections:
		h.ToggleSearchOnlySelections()
	default:
		return false
	}
	return true
}

// applyFindListNav applies Up/Down/PgUp/PgDn/Ctrl+Home/Ctrl+End to the ranked
// result list when it has focus, updating scroll and the nav-idle debounce.
// Returns false (no-op) when the list isn't focused, is empty, or key isn't a
// recognized list-navigation key.
func (h *Handler) applyFindListNav(key tcell.Key, mods tcell.ModMask) bool {
	st := &h.model.FindDialog
	if st.Focus != 0 || len(st.Ranked) == 0 {
		return false
	}
	sel, ok := dialog.ListNavKeySelection(key, mods, st.Selected, len(st.Ranked), max(1, h.findDialogListRows()-1))
	if !ok {
		return false
	}
	st.Selected = sel
	dialog.EnsureFindListScroll(st, h.findDialogListRows())
	h.armFindNavIdleTimer()
	return true
}

// tryFindDialogActionKey dispatches the keymap.ActionFind* bindings looked up via
// h.keysFindDialog (select/unselect all, group select/unselect, open in
// primary/secondary). Returns true when the event matched one of these actions.
func (h *Handler) tryFindDialogActionKey(event *tcell.EventKey) bool {
	if h.keysFindDialog == nil {
		return false
	}
	id, ok := h.keysFindDialog.Lookup(event)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFindView:
		h.OpenSelectedFullscreenPreview()
	case keymap.ActionFindUnselectAll:
		h.findDialogUnselectAll()
	case keymap.ActionFindSelectAll:
		h.findDialogSelectAll()
	case keymap.ActionFindSelectGroup:
		h.host.OpenGroupSelectDialog(GroupSelectModeSelect, true)
	case keymap.ActionFindUnselectGroup:
		h.host.OpenGroupSelectDialog(GroupSelectModeUnselect, true)
	case keymap.ActionFindOpenInPrimary:
		h.OpenSelectedInPrimary()
	case keymap.ActionFindOpenInSecondary:
		h.OpenSelectedInSecondary()
	default:
		return false
	}
	return true
}

func (h *Handler) HandleDialogKey(event *tcell.EventKey) {
	st := &h.model.FindDialog
	if h.tryFindDialogActionKey(event) {
		return
	}
	if id, ok := h.keys.Lookup(event); ok && id == keymap.ActionPanelSelectToggle {
		h.findDialogToggleSelectionAndAdvance()
		return
	}
	if dialog.AltDialogOK(event) {
		h.ActivateDialogOK()
		return
	}
	if dialog.AltDialogCancel(event) {
		h.CloseDialog()
		return
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if field := findToggleFieldForRune(event.Rune()); field != findToggleNone {
			if field != findToggleSelections || st.FindDialogHasSelectionsCheckbox() {
				h.applyFindToggle(field)
			}
			return
		}
	}

	if st.Focus == 0 {
		onChange := func() {
			st.Selected = 0
			h.scheduleFindRank(-h.config.UI.Find.QueryDebounceMS)
		}
		if h.host.HandleScrollingQueryKey(event, true, h.host.FindDialogScrollingQuery(st, h.host.FindDialogQueryWidth(), onChange)) {
			return
		}
	}

	switch event.Key() {
	case tcell.KeyInsert:
		h.findDialogToggleSelectionAndAdvance()
	case tcell.KeyEsc:
		h.CloseDialog()
	case tcell.KeyEnter:
		if st.Focus == st.FindDialogCancelFocus() {
			h.CloseDialog()
		} else if !h.applyFindToggle(h.findToggleFieldForFocus(st.Focus)) {
			h.ActivateDialogOK()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := dialog.FindDialogNavFocusKey(st.Focus, st.FindDialogHasSelectionsCheckbox(), event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				dialog.EnsureFindListScroll(st, h.findDialogListRows())
			}
			break
		}
		h.applyFindListNav(event.Key(), event.Modifiers())
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		h.applyFindListNav(event.Key(), event.Modifiers())
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		if st.Focus == 0 {
			break
		}
		if field := findToggleFieldForRune(event.Rune()); field != findToggleNone {
			if field == h.findToggleFieldForFocus(st.Focus) {
				h.applyFindToggle(field)
			}
			break
		}
		switch dialog.DialogButtonRune(event.Rune()) {
		case dialog.ButtonRuneOK:
			h.ActivateDialogOK()
		case dialog.ButtonRuneCancel:
			h.CloseDialog()
		case dialog.ButtonRuneToggle:
			if !h.applyFindToggle(h.findToggleFieldForFocus(st.Focus)) {
				switch st.Focus {
				case st.FindDialogOKFocus():
					h.ActivateDialogOK()
				case st.FindDialogCancelFocus():
					h.CloseDialog()
				}
			}
		}
	}
}

// matchingFindPaths returns the absolute paths of entries at indices whose basename
// matches matcher, honoring filesOnly/dirsOnly. Shared filter pipeline for both
// ApplyGroupSelect branches (select also filters out already-marked paths itself).
func matchingFindPaths(st *dialog.FindDialogState, indices []int, filesOnly, dirsOnly bool, matcher panel.GroupMatcher) []string {
	if indices == nil {
		paths := make([]string, 0, 256)
		for i := 0; i < st.IndexedCount; i++ {
			ent, ok := st.FindEntryAt(i)
			if !ok {
				continue
			}
			if filesOnly && ent.IsDir {
				continue
			}
			if dirsOnly && !ent.IsDir {
				continue
			}
			path := findEntryAbsPath(st, ent)
			if path == "" || !matcher.Match(filepath.Base(path)) {
				continue
			}
			paths = append(paths, path)
		}
		return paths
	}
	paths := make([]string, 0, len(indices))
	for _, entIdx := range indices {
		ent, ok := st.FindEntryAt(entIdx)
		if !ok {
			continue
		}
		if filesOnly && ent.IsDir {
			continue
		}
		if dirsOnly && !ent.IsDir {
			continue
		}
		path := findEntryAbsPath(st, ent)
		if path == "" {
			continue
		}
		if !matcher.Match(filepath.Base(path)) {
			continue
		}
		paths = append(paths, path)
	}
	return paths
}

// CountGroupMatches reports how many find results matching req.Pattern have marked state equal
// to marked — i.e. how many results ApplyGroupSelect(select) (marked=false) or (unselect)
// (marked=true) would actually change, split into files and directories, for the group-select
// dialog's live result preview. Returns (0, 0) for an empty or invalid pattern.
func (h *Handler) CountGroupMatches(req GroupSelectRequest, marked bool) (files, dirs int) {
	if req.Pattern == "" {
		return 0, 0
	}
	matcher, err := panel.NewGroupMatcher(req.Pattern, req.PatternMode, req.CaseSensitive)
	if err != nil {
		return 0, 0
	}
	st := &h.model.FindDialog
	indices := h.findDialogResultIndices(st)
	matched := matchingFindPaths(st, indices, req.FilesOnly, req.DirsOnly, matcher)
	isDir := h.findPathIsDir(st)
	for _, path := range matched {
		if st.MarkedPaths[path] != marked {
			continue
		}
		if isDir(path) {
			dirs++
		} else {
			files++
		}
	}
	return files, dirs
}

// ApplyGroupSelect marks or unmarks full-corpus find results whose basename matches pattern.
func (h *Handler) ApplyGroupSelect(req GroupSelectRequest) {
	st := &h.model.FindDialog
	pattern := req.Pattern
	if pattern == "" {
		return
	}
	matcher, err := panel.NewGroupMatcher(pattern, req.PatternMode, req.CaseSensitive)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	indices := h.findDialogResultIndices(st)
	if req.Mode == GroupSelectModeSelect {
		walkOrder := len(st.MarkedPaths) == 0
		if st.MarkedPaths == nil {
			st.MarkedPaths = make(map[string]bool, len(indices))
		}
		matched := matchingFindPaths(st, indices, req.FilesOnly, req.DirsOnly, matcher)
		paths := make([]string, 0, len(matched))
		for _, path := range matched {
			if !st.MarkedPaths[path] {
				paths = append(paths, path)
			}
		}
		conflicts := false
		if len(paths) > 0 {
			isDir := h.findPathIsDir(st)
			if walkOrder {
				conflicts = panel.BulkApplySelectionAddsWalkOrder(st.MarkedPaths, paths, isDir)
			} else {
				conflicts = panel.BulkApplySelectionAdds(st.MarkedPaths, paths, isDir)
			}
		}
		if len(st.MarkedPaths) == 0 {
			st.MarkedPaths = nil
		}
		switch {
		case len(matched) == 0:
			h.host.SetTransientMessage("No matches", ui.MessageUrgencyWarn)
		case conflicts:
			h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		default:
			h.host.SetTransientMessage(fmt.Sprintf("Selected matching %q", pattern), ui.MessageUrgencyInfo)
		}
		st.InvalidateMarkedSelectionDerived()
		return
	}
	unmatched := matchingFindPaths(st, indices, req.FilesOnly, req.DirsOnly, matcher)
	for _, path := range unmatched {
		delete(st.MarkedPaths, path)
	}
	if len(st.MarkedPaths) == 0 {
		st.MarkedPaths = nil
	}
	st.InvalidateMarkedSelectionDerived()
	if len(unmatched) == 0 {
		h.host.SetTransientMessage("No matches", ui.MessageUrgencyWarn)
	} else {
		h.host.SetTransientMessage(fmt.Sprintf("Unselected matching %q", pattern), ui.MessageUrgencyInfo)
	}
}
