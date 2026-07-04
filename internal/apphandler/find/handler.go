package find

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	findpkg "github.com/paranoidi/paras-commander/internal/find"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func New(d Deps) *Handler {
	h := &Handler{
		host:           d.Host,
		screen:         d.Screen,
		model:          d.Model,
		config:         d.Config,
		keys:           d.Keys,
		keysFindDialog: d.KeysFindDialog,
		rankReady:      make(chan rankResult, 1),
		rankWorkCh:     make(chan rankInput, 1),
	}
	go h.rankWorkerLoop()
	return h
}

// WakePayload wakes PollEvent when find indexer batches arrive or finish.
type WakePayload struct {
	Finished bool
	WalkErr  string // set when a walk goroutine exits with an error (applied on main thread)
}

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
		ShowHidden:                 p.ShowHidden,
		StayOnCurrentVolume:        true,
		ListingDevice:              p.ListingDevice,
		ListingDeviceValid:         p.ListingDeviceValid,
		ShowSearchSelectionsOption: len(selRoots) > 0,
		SearchOnlySelections:       len(selRoots) > 0,
		SelectionDirRoots:          selRoots,
		Focus:                      0,
		Indexing:                   true,
	}
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

func (h *Handler) startFindIndexer() {
	st := &h.model.FindDialog
	if !st.Open || st.RootPath == "" {
		return
	}
	h.sessionMu.Lock()
	h.batchCh = make(chan []findpkg.Entry, 32)
	h.walks = make(map[string]*walk)
	h.indexedPaths = make(map[string]struct{})
	h.completedRoots = make(map[string]struct{})
	h.sessionMu.Unlock()

	st.Indexing = true
	st.IndexDone = false
	st.IndexErr = ""
	for _, root := range h.findScopeRoots(st) {
		h.startFindWalk(root, false)
	}
}

func (h *Handler) restartFindIndexer() {
	h.stopFindIndexer()
	h.cancelPendingRank()
	st := &h.model.FindDialog
	if !st.Open || st.RootPath == "" {
		return
	}
	st.IndexErr = ""
	st.Entries = nil
	st.PathIsDir = nil
	st.IndexedCount = 0
	st.Ranked = nil
	st.MatchRanges = nil
	st.Selected = 0
	st.ListScroll = 0
	st.RankPending = false
	h.startFindIndexer()
}

func (h *Handler) stopFindIndexer() {
	h.sessionMu.Lock()
	walks := h.walks
	ch := h.batchCh
	h.walks = nil
	h.batchCh = nil
	h.indexedPaths = nil
	h.completedRoots = nil
	h.sessionMu.Unlock()
	for _, w := range walks {
		if w != nil && w.sess != nil {
			w.sess.Close()
		}
	}
	if ch != nil {
		close(ch)
		for range ch {
		}
	}
}

func (h *Handler) findWalkActive(root string) bool {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	_, ok := h.walks[filepath.Clean(root)]
	return ok
}

func (h *Handler) findWalkCompleted(root string) bool {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	_, ok := h.completedRoots[filepath.Clean(root)]
	return ok
}

func (h *Handler) findSelectionSkipRoots(st *dialog.FindDialogState) []string {
	if !st.ShowSearchSelectionsOption {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	h.sessionMu.Lock()
	for root := range h.walks {
		r := filepath.Clean(root)
		if r == filepath.Clean(st.RootPath) {
			continue
		}
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	for root := range h.completedRoots {
		r := filepath.Clean(root)
		if r == filepath.Clean(st.RootPath) {
			continue
		}
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	h.sessionMu.Unlock()
	for _, r := range st.SelectionDirRoots {
		r = filepath.Clean(r)
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

func (h *Handler) startFindWalk(root string, skipIndexedSelectionSubtrees bool) {
	st := &h.model.FindDialog
	root = filepath.Clean(root)
	if root == "" {
		return
	}
	h.sessionMu.Lock()
	if h.walks == nil {
		h.walks = make(map[string]*walk)
	}
	if _, exists := h.walks[root]; exists {
		h.sessionMu.Unlock()
		return
	}
	if _, done := h.completedRoots[root]; done {
		h.sessionMu.Unlock()
		return
	}
	ch := h.batchCh
	h.sessionMu.Unlock()
	if ch == nil {
		return
	}

	volumeSkip := diskusage.ComposeListingVolumeIgnore(nil, h.findVolumeGate(st))
	var shouldSkip func(string) bool
	if skipIndexedSelectionSubtrees {
		skipRoots := h.findSelectionSkipRoots(st)
		shouldSkip = func(abs string) bool {
			if volumeSkip != nil && volumeSkip(abs) {
				return true
			}
			abs = filepath.Clean(abs)
			for _, sr := range skipRoots {
				if panel.IsStrictPathDescendant(sr, abs) {
					return true
				}
			}
			return false
		}
	} else {
		shouldSkip = volumeSkip
	}

	sess := findpkg.Start(context.Background(), root, findpkg.Options{
		ShowHidden:    st.ShowHidden,
		Gitignore:     h.host.GitignoreCache(),
		ShouldSkipDir: shouldSkip,
	})
	h.sessionMu.Lock()
	if h.batchCh != ch {
		h.sessionMu.Unlock()
		sess.Close()
		return
	}
	h.walks[root] = &walk{root: root, sess: sess}
	st.Indexing = true
	st.IndexDone = false
	h.sessionMu.Unlock()
	go h.readFindSession(sess, ch, root)
}

func (h *Handler) readFindSession(sess *findpkg.Session, ch chan []findpkg.Entry, root string) {
	for batch := range sess.Results() {
		h.sessionMu.Lock()
		active := h.batchCh == ch
		h.sessionMu.Unlock()
		if !active {
			return
		}
		select {
		case ch <- batch:
		default:
			ch <- batch
		}
		// Only post one WakePayload at a time. PollUpdates drains ALL queued batches in a
		// single call, so multiple WakePayloads just flood the event queue and push key
		// events further back, causing visible input lag.
		if h.wakePending.CompareAndSwap(false, true) {
			_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{}))
		}
	}
	<-sess.Done()
	err := sess.Err()
	h.sessionMu.Lock()
	if h.walks != nil {
		delete(h.walks, root)
	}
	if h.completedRoots == nil {
		h.completedRoots = make(map[string]struct{})
	}
	h.completedRoots[root] = struct{}{}
	h.sessionMu.Unlock()
	payload := WakePayload{Finished: true}
	if err != nil {
		payload.WalkErr = err.Error()
	}
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(payload))
}

func (h *Handler) findAllWalksDone() bool {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	return len(h.walks) == 0
}

func (h *Handler) updateFindIndexingState() {
	st := &h.model.FindDialog
	if !st.Open {
		return
	}
	if h.findAllWalksDone() {
		st.Indexing = false
		if !st.IndexDone {
			st.IndexDone = true
			h.indexedPaths = nil
		}
	} else {
		st.Indexing = true
		st.IndexDone = false
	}
}

func findRelLine(displayRoot, absPath string) string {
	rel, err := filepath.Rel(displayRoot, absPath)
	if err != nil {
		return filepath.ToSlash(absPath)
	}
	return filepath.ToSlash(rel)
}

func (h *Handler) appendFindBatch(st *dialog.FindDialogState, batch []findpkg.Entry) {
	if len(batch) == 0 {
		return
	}
	h.sessionMu.Lock()
	if h.indexedPaths == nil {
		h.indexedPaths = make(map[string]struct{})
	}
	h.sessionMu.Unlock()
	for _, e := range batch {
		p := filepath.Clean(e.Path)
		if p == "" {
			continue
		}
		h.sessionMu.Lock()
		if _, dup := h.indexedPaths[p]; dup {
			h.sessionMu.Unlock()
			continue
		}
		h.indexedPaths[p] = struct{}{}
		h.sessionMu.Unlock()
		st.Entries = append(st.Entries, dialog.FindEntry{
			RelLine: findRelLine(st.RootPath, p),
			IsDir:   e.IsDir,
			Type:    e.Type,
			Size:    e.Size,
		})
		if st.PathIsDir == nil {
			st.PathIsDir = make(map[string]bool)
		}
		st.PathIsDir[p] = e.IsDir
		if !e.IsDir && e.Size > 0 {
			if st.PathSize == nil {
				st.PathSize = make(map[string]int64)
			}
			st.PathSize[p] = e.Size
		}
	}
	st.IndexedCount = len(st.Entries)
}

func rebuildFindPathIndex(st *dialog.FindDialogState) {
	if len(st.Entries) == 0 {
		st.PathIsDir = nil
		st.PathSize = nil
		return
	}
	isDir := make(map[string]bool, len(st.Entries))
	sizes := make(map[string]int64)
	root := st.RootPath
	for _, e := range st.Entries {
		abs := filepath.Clean(e.AbsPath(root))
		if abs == "" {
			continue
		}
		isDir[abs] = e.IsDir
		if !e.IsDir && e.Size > 0 {
			sizes[abs] = e.Size
		}
	}
	st.PathIsDir = isDir
	if len(sizes) == 0 {
		st.PathSize = nil
	} else {
		st.PathSize = sizes
	}
}

func findEntryAbsPath(st *dialog.FindDialogState, ent dialog.FindEntry) string {
	return filepath.Clean(ent.AbsPath(st.RootPath))
}

func (h *Handler) findPathIsDir(st *dialog.FindDialogState) func(string) bool {
	return func(path string) bool {
		if st.PathIsDir == nil {
			return false
		}
		return st.PathIsDir[filepath.Clean(path)]
	}
}

func pathInFindSelectionScope(path string, roots []string) bool {
	path = filepath.Clean(path)
	for _, r := range roots {
		r = filepath.Clean(r)
		if path == r || panel.IsStrictPathDescendant(r, path) {
			return true
		}
	}
	return false
}

func (h *Handler) filterFindEntriesToSelectionScope() {
	st := &h.model.FindDialog
	if len(st.SelectionDirRoots) == 0 {
		return
	}
	filtered := st.Entries[:0]
	h.sessionMu.Lock()
	h.indexedPaths = make(map[string]struct{})
	h.sessionMu.Unlock()
	for _, e := range st.Entries {
		if pathInFindSelectionScope(e.AbsPath(st.RootPath), st.SelectionDirRoots) {
			filtered = append(filtered, e)
			h.sessionMu.Lock()
			h.indexedPaths[e.AbsPath(st.RootPath)] = struct{}{}
			h.sessionMu.Unlock()
		}
	}
	st.Entries = filtered
	st.IndexedCount = len(st.Entries)
	rebuildFindPathIndex(st)
	h.syncFindDialogRanks()
}

func (h *Handler) stopFindWalksOutsideSelectionScope() {
	st := &h.model.FindDialog
	panelRoot := filepath.Clean(st.RootPath)
	allowed := make(map[string]struct{}, len(st.SelectionDirRoots))
	for _, r := range st.SelectionDirRoots {
		allowed[filepath.Clean(r)] = struct{}{}
	}
	h.sessionMu.Lock()
	var stop []*walk
	for root, w := range h.walks {
		root = filepath.Clean(root)
		if root == panelRoot {
			stop = append(stop, w)
			continue
		}
		if _, ok := allowed[root]; !ok {
			stop = append(stop, w)
		}
	}
	for _, w := range stop {
		delete(h.walks, w.root)
		delete(h.completedRoots, w.root)
	}
	h.sessionMu.Unlock()
	for _, w := range stop {
		if w.sess != nil {
			w.sess.Close()
		}
	}
}

func (h *Handler) widenFindIndexer() {
	st := &h.model.FindDialog
	panelRoot := filepath.Clean(st.RootPath)
	if h.findWalkActive(panelRoot) || h.findWalkCompleted(panelRoot) {
		h.updateFindIndexingState()
		return
	}
	h.startFindWalk(panelRoot, true)
	h.updateFindIndexingState()
}

func (h *Handler) narrowFindIndexer() {
	h.stopFindWalksOutsideSelectionScope()
	h.filterFindEntriesToSelectionScope()
	h.updateFindIndexingState()
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

func (h *Handler) PollUpdates(payload WakePayload) bool {
	needRender := false
	needSync := false
	// Clear the dedup flag so the next batch from readFindSession can post a new WakePayload.
	h.wakePending.Store(false)
	h.sessionMu.Lock()
	ch := h.batchCh
	h.sessionMu.Unlock()
	if ch != nil {
		for {
			select {
			case batch, ok := <-ch:
				if !ok {
					goto drained
				}
				st := &h.model.FindDialog
				if !st.Open {
					continue
				}
				h.appendFindBatch(st, batch)
				needSync = true
				needRender = true
			default:
				goto drained
			}
		}
	}
drained:
	if needSync {
		throttle := 0
		st := &h.model.FindDialog
		if st.Open && st.Indexing {
			throttle = config.DefaultFindIndexingRankThrottleMS
		}
		h.scheduleFindRank(throttle)
	}
	st := &h.model.FindDialog
	if st.Open && payload.WalkErr != "" {
		st.IndexErr = payload.WalkErr
		needRender = true
	}
	if st.Open && (st.Indexing || payload.Finished) {
		if h.findAllWalksDone() {
			if st.Indexing || !st.IndexDone {
				st.Indexing = false
				st.IndexDone = true
				h.indexedPaths = nil
				needRender = true
			}
		} else if !st.Indexing {
			st.Indexing = true
			st.IndexDone = false
			needRender = true
		}
	}
	return needRender
}

func (h *Handler) syncFindDialogRanks() {
	st := &h.model.FindDialog
	if !st.Open {
		return
	}
	ranked, matchRanges := dialog.RankFindEntries(
		st.Entries,
		st.Query,
		st.OnlyDirectories,
		st.OnlyFiles,
		h.config.CaseInsensitiveFilter,
	)
	st.Ranked = ranked
	st.MatchRanges = matchRanges
	h.rankMu.Lock()
	st.FullRanked = ranked
	st.FullRankedGen = h.rankGen
	h.rankMu.Unlock()
	st.FullRankedEntriesLen = len(st.Entries)
	st.FullRankedOnlyDirs = st.OnlyDirectories
	st.FullRankedOnlyFiles = st.OnlyFiles
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

// rankWorkerLoop is the single rank worker goroutine. It processes one rank computation at a time,
// eliminating the memory spike from many concurrent goroutines each holding a large snapshot.
func (h *Handler) rankWorkerLoop() {
	for input := range h.rankWorkCh {
		h.doRank(input)
	}
}

// sendRankWork delivers input to the rank worker, replacing any pending (unconsumed) input.
// rankSendMu ensures concurrent callers (main thread + timer goroutine) don't interleave.
func (h *Handler) sendRankWork(input rankInput) {
	h.rankSendMu.Lock()
	defer h.rankSendMu.Unlock()
	// Drain any pending input that hasn't been picked up yet (frees its snapshot memory).
	select {
	case <-h.rankWorkCh:
	default:
	}
	h.rankWorkCh <- input
}

// buildRankInput takes a compact snapshot from the main-thread dialog state.
// lines/isDirs share string data with st.Entries; no extra string allocations.
func (h *Handler) buildRankInput(gen int, st *dialog.FindDialogState) rankInput {
	n := len(st.Entries)
	lines := make([]string, n)
	isDirs := make([]bool, n)
	for i, e := range st.Entries {
		lines[i] = e.RelLine
		isDirs[i] = e.IsDir
	}
	return rankInput{
		gen:       gen,
		lines:     lines,
		isDirs:    isDirs,
		query:     st.Query,
		onlyDirs:  st.OnlyDirectories,
		onlyFiles: st.OnlyFiles,
		opts:      search.Options{CaseInsensitive: h.config.CaseInsensitiveFilter},
	}
}

// scheduleFindRank takes a snapshot on the main thread and schedules a rank computation.
//
// ms == 0: send immediately, cancel any pending timer/wake.
//
// ms > 0: throttle mode (for indexing batches). Dispatch immediately if the cooldown has
// elapsed; otherwise ensure a single trailing-edge timer is running (never reset it — that
// would turn throttle into debounce). The timer posts ThrottleRankWakePayload so the main
// thread re-snapshots with the freshest data in HandleThrottleRankWake.
//
// ms < 0: debounce mode (for query-typing). Reset the timer on every call so the rank fires
// abs(ms) milliseconds after the last keystroke. Uses ThrottleRankWakePayload as the wake.
//
// rankGen is incremented only when work is actually dispatched (immediate paths); the timer
// paths leave gen unchanged so in-flight computations are not prematurely cancelled.
//
// Called only from the main thread.
func (h *Handler) scheduleFindRank(ms int) {
	st := &h.model.FindDialog
	if !st.Open {
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
		h.sendRankWork(h.buildRankInput(gen, st))
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
		h.sendRankWork(h.buildRankInput(gen, st))
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
	if !st.Open {
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
	h.sendRankWork(h.buildRankInput(gen, st))
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
	h.sendRankWork(h.buildRankInput(gen, st))
	return true
}

// armFindNavIdleTimer resets the navigation-idle debounce timer. While the timer is
// running ApplyPendingRank defers applying background rank updates to keep the view
// stable during fast navigation. Called only from the main thread.
func (h *Handler) armFindNavIdleTimer() {
	if h.findNavTimer != nil {
		h.findNavTimer.Stop()
		h.findNavTimer = nil
	}
	idleMS := h.config.UI.FindListNavIdleMS
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

// doRank is called by the rank worker goroutine. It computes the ranked result for the given
// input snapshot, then delivers it via rankReady and wakes the event loop.
func (h *Handler) doRank(input rankInput) {
	h.rankMu.Lock()
	if h.rankGen != input.gen {
		h.rankMu.Unlock()
		return
	}
	h.rankMu.Unlock()

	q := search.Parse(input.query)
	maxResults := h.config.UI.FindMaxResults

	var result rankResult
	result.gen = input.gen

	if q.Empty() {
		result.fullRanked = filterFindRankIndices(allFindEntryIndices(len(input.lines)), input.onlyDirs, input.onlyFiles, input.isDirs)
		displayN := len(result.fullRanked)
		if maxResults > 0 && displayN > maxResults {
			displayN = maxResults
		}
		result.ranked = append([]int(nil), result.fullRanked[:displayN]...)
	} else {
		rawFull, cancelled := q.RankCancellable(input.lines, input.opts, func() bool {
			h.rankMu.Lock()
			stale := h.rankGen != input.gen
			h.rankMu.Unlock()
			return stale
		})
		if cancelled {
			return // new query superseded this one; worker will pick up the next input
		}
		result.fullRanked = filterFindRankIndices(rankedIndicesFromResults(rawFull), input.onlyDirs, input.onlyFiles, input.isDirs)

		raw := rawFull
		if maxResults > 0 && len(raw) > maxResults {
			raw = raw[:maxResults]
		}
		result.ranked = filterFindRankIndices(rankedIndicesFromResults(raw), input.onlyDirs, input.onlyFiles, input.isDirs)
		result.matchRanges = make(map[int][]search.Range)
		for _, r := range raw {
			idx := r.Index
			if idx >= 0 && idx < len(input.lines) && len(r.Result.Ranges) > 0 {
				if input.onlyDirs && (idx >= len(input.isDirs) || !input.isDirs[idx]) {
					continue
				}
				if input.onlyFiles && (idx >= len(input.isDirs) || input.isDirs[idx]) {
					continue
				}
				result.matchRanges[idx] = r.Result.Ranges
			}
		}
		if len(result.matchRanges) == 0 {
			result.matchRanges = nil
		}
	}

	h.rankMu.Lock()
	if h.rankGen != input.gen {
		h.rankMu.Unlock()
		return
	}
	// Drain any stale result before delivering the fresh one.
	select {
	case <-h.rankReady:
	default:
	}
	h.rankReady <- result
	h.rankMu.Unlock()

	_ = h.screen.PostEvent(tcell.NewEventInterrupt(RankWakePayload{}))
}

func allFindEntryIndices(n int) []int {
	if n == 0 {
		return nil
	}
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func rankedIndicesFromResults(raw []search.RankedResult) []int {
	out := make([]int, len(raw))
	for i, r := range raw {
		out[i] = r.Index
	}
	return out
}

func filterFindRankIndices(indices []int, onlyDirs, onlyFiles bool, isDirs []bool) []int {
	if !onlyDirs && !onlyFiles {
		out := append([]int(nil), indices...)
		return out
	}
	filtered := make([]int, 0, len(indices))
	for _, idx := range indices {
		if idx < 0 || idx >= len(isDirs) {
			continue
		}
		if onlyDirs && !isDirs[idx] {
			continue
		}
		if onlyFiles && isDirs[idx] {
			continue
		}
		filtered = append(filtered, idx)
	}
	return filtered
}

// ApplyPendingRank applies the latest completed rank result (if any) to the dialog state.
// Returns true if a result was applied and the UI needs a re-render.
// Defers applying when the user is actively navigating the list (nav-idle debounce).
// Called only from the main thread.
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
	var result rankResult
	var ok bool
	select {
	case result = <-h.rankReady:
		ok = true
	default:
	}
	h.rankMu.Unlock()

	if !ok || result.gen != gen {
		return false
	}

	// Remember the currently selected entry so we can restore the selection after re-ranking.
	var selectedRelLine string
	if st.Selected >= 0 && st.Selected < len(st.Ranked) {
		if entIdx := st.Ranked[st.Selected]; entIdx >= 0 && entIdx < len(st.Entries) {
			selectedRelLine = st.Entries[entIdx].RelLine
		}
	}

	st.Ranked = result.ranked
	st.MatchRanges = result.matchRanges // sparse map; nil for empty query
	st.FullRanked = result.fullRanked
	st.FullRankedGen = gen
	st.FullRankedEntriesLen = len(st.Entries)
	st.FullRankedOnlyDirs = st.OnlyDirectories
	st.FullRankedOnlyFiles = st.OnlyFiles

	// Restore selection by identity (RelLine) so the same item stays highlighted.
	if selectedRelLine != "" {
		for i, entIdx := range st.Ranked {
			if entIdx >= 0 && entIdx < len(st.Entries) && st.Entries[entIdx].RelLine == selectedRelLine {
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
	if entIdx < 0 || entIdx >= len(st.Entries) {
		return "", false
	}
	ent := st.Entries[entIdx]
	path := ent.AbsPath(st.RootPath)

	dir, name := path, ""
	if !ent.IsDir {
		dir = filepath.Clean(filepath.Dir(path))
		name = filepath.Base(path)
	}
	if err := h.host.NavigatePanelToDirectory(panelID, dir, name); err != nil {
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

func (h *Handler) findDialogResultIndices(st *dialog.FindDialogState) []int {
	q := search.Parse(st.Query)
	if q.Empty() {
		indices := make([]int, 0, len(st.Entries))
		for i, e := range st.Entries {
			if st.OnlyDirectories && !e.IsDir {
				continue
			}
			if st.OnlyFiles && e.IsDir {
				continue
			}
			indices = append(indices, i)
		}
		return indices
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
	ranked, _ := dialog.RankFindEntries(
		st.Entries,
		st.Query,
		st.OnlyDirectories,
		st.OnlyFiles,
		h.config.CaseInsensitiveFilter,
	)
	return ranked
}

func (h *Handler) findDialogSelectAll() {
	st := &h.model.FindDialog
	indices := h.findDialogResultIndices(st)
	if len(indices) == 0 {
		return
	}
	walkOrder := len(st.MarkedPaths) == 0
	if st.MarkedPaths == nil {
		st.MarkedPaths = make(map[string]bool, len(indices))
	}
	paths := make([]string, 0, len(indices))
	for _, entIdx := range indices {
		if entIdx < 0 || entIdx >= len(st.Entries) {
			continue
		}
		path := findEntryAbsPath(st, st.Entries[entIdx])
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
	if entIdx < 0 || entIdx >= len(st.Entries) {
		return
	}
	path := findEntryAbsPath(st, st.Entries[entIdx])
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

func (h *Handler) HandleDialogKey(event *tcell.EventKey) {
	st := &h.model.FindDialog
	if h.keysFindDialog != nil {
		if id, ok := h.keysFindDialog.Lookup(event); ok {
			switch id {
			case keymap.ActionFindUnselectAll:
				h.findDialogUnselectAll()
				return
			case keymap.ActionFindSelectAll:
				h.findDialogSelectAll()
				return
			case keymap.ActionFindSelectGroup:
				h.host.OpenGroupSelectDialog("select", true)
				return
			case keymap.ActionFindUnselectGroup:
				h.host.OpenGroupSelectDialog("unselect", true)
				return
			case keymap.ActionFindOpenInPrimary:
				h.OpenSelectedInPrimary()
				return
			case keymap.ActionFindOpenInSecondary:
				h.OpenSelectedInSecondary()
				return
			}
		}
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
		switch event.Rune() {
		case 'v', 'V':
			h.ToggleStayOnVolume()
			return
		case 'd', 'D':
			h.ToggleOnlyDirectories()
			return
		case 'l', 'L':
			h.ToggleOnlyFiles()
			return
		case 's', 'S':
			if st.FindDialogHasSelectionsCheckbox() {
				h.ToggleSearchOnlySelections()
				st.Focus = st.FindDialogSelectionsFocus()
			}
			return
		}
	}

	if st.Focus == 0 {
		onChange := func() {
			st.Selected = 0
			h.scheduleFindRank(-h.config.UI.FindQueryDebounceMS)
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
		switch st.Focus {
		case st.FindDialogCancelFocus():
			h.CloseDialog()
		case st.FindDialogStayOnVolumeFocus():
			h.ToggleStayOnVolume()
		case st.FindDialogOnlyDirsFocus():
			h.ToggleOnlyDirectories()
		case st.FindDialogOnlyFilesFocus():
			h.ToggleOnlyFiles()
		case st.FindDialogSelectionsFocus():
			h.ToggleSearchOnlySelections()
		default:
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
		if st.Focus == 0 && len(st.Ranked) > 0 {
			switch event.Key() {
			case tcell.KeyUp:
				st.Selected = dialog.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
				dialog.EnsureFindListScroll(st, h.findDialogListRows())
				h.armFindNavIdleTimer()
			case tcell.KeyDown:
				st.Selected = dialog.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
				dialog.EnsureFindListScroll(st, h.findDialogListRows())
				h.armFindNavIdleTimer()
			}
		}
	case tcell.KeyHome:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = 0
			dialog.EnsureFindListScroll(st, h.findDialogListRows())
			h.armFindNavIdleTimer()
		}
	case tcell.KeyEnd:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			dialog.EnsureFindListScroll(st, h.findDialogListRows())
			h.armFindNavIdleTimer()
		}
	case tcell.KeyPgUp:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, h.findDialogListRows()-1)
			st.Selected = dialog.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -step)
			dialog.EnsureFindListScroll(st, h.findDialogListRows())
			h.armFindNavIdleTimer()
		}
	case tcell.KeyPgDn:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, h.findDialogListRows()-1)
			st.Selected = dialog.ListClampedSelectionDelta(st.Selected, len(st.Ranked), step)
			dialog.EnsureFindListScroll(st, h.findDialogListRows())
			h.armFindNavIdleTimer()
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		if st.Focus == 0 {
			break
		}
		switch event.Rune() {
		case 'v', 'V':
			if st.Focus == st.FindDialogStayOnVolumeFocus() {
				h.ToggleStayOnVolume()
			}
		case 'd', 'D':
			if st.Focus == st.FindDialogOnlyDirsFocus() {
				h.ToggleOnlyDirectories()
			}
		case 'l', 'L':
			if st.Focus == st.FindDialogOnlyFilesFocus() {
				h.ToggleOnlyFiles()
			}
		case 's', 'S':
			if st.Focus == st.FindDialogSelectionsFocus() {
				h.ToggleSearchOnlySelections()
			}
		case 'o', 'O':
			h.ActivateDialogOK()
		case 'c', 'C':
			h.CloseDialog()
		case ' ':
			switch st.Focus {
			case st.FindDialogStayOnVolumeFocus():
				h.ToggleStayOnVolume()
			case st.FindDialogOnlyDirsFocus():
				h.ToggleOnlyDirectories()
			case st.FindDialogOnlyFilesFocus():
				h.ToggleOnlyFiles()
			case st.FindDialogSelectionsFocus():
				h.ToggleSearchOnlySelections()
			case st.FindDialogOKFocus():
				h.ActivateDialogOK()
			case st.FindDialogCancelFocus():
				h.CloseDialog()
			}
		}
	}
}

// ApplyGroupSelect marks or unmarks full-corpus find results whose basename matches pattern.
func (h *Handler) ApplyGroupSelect(mode, pattern string, filesOnly, dirsOnly, caseSensitive bool, patternMode panel.GroupPatternMode) {
	st := &h.model.FindDialog
	if pattern == "" {
		return
	}
	matcher, err := panel.NewGroupMatcher(pattern, patternMode, caseSensitive)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	indices := h.findDialogResultIndices(st)
	if mode == "select" {
		walkOrder := len(st.MarkedPaths) == 0
		if st.MarkedPaths == nil {
			st.MarkedPaths = make(map[string]bool, len(indices))
		}
		paths := make([]string, 0, len(indices))
		for _, entIdx := range indices {
			if entIdx < 0 || entIdx >= len(st.Entries) {
				continue
			}
			ent := st.Entries[entIdx]
			if filesOnly && ent.IsDir {
				continue
			}
			if dirsOnly && !ent.IsDir {
				continue
			}
			path := findEntryAbsPath(st, ent)
			if path == "" || st.MarkedPaths[path] {
				continue
			}
			name := filepath.Base(path)
			if !matcher.Match(name) {
				continue
			}
			paths = append(paths, path)
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
		if conflicts {
			h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		} else {
			h.host.SetTransientMessage(fmt.Sprintf("Selected matching %q", pattern), ui.MessageUrgencyInfo)
		}
		st.InvalidateMarkedSelectionDerived()
		return
	}
	for _, entIdx := range indices {
		if entIdx < 0 || entIdx >= len(st.Entries) {
			continue
		}
		ent := st.Entries[entIdx]
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
		name := filepath.Base(path)
		if !matcher.Match(name) {
			continue
		}
		delete(st.MarkedPaths, path)
	}
	if len(st.MarkedPaths) == 0 {
		st.MarkedPaths = nil
	}
	st.InvalidateMarkedSelectionDerived()
	h.host.SetTransientMessage(fmt.Sprintf("Unselected matching %q", pattern), ui.MessageUrgencyInfo)
}
