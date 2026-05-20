package find

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	findpkg "github.com/paranoidi/paras-commander/internal/find"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func New(d Deps) *Handler {
	return &Handler{
		host:   d.Host,
		screen: d.Screen,
		model:  d.Model,
		config: d.Config,
		keys:   d.Keys,
	}
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
	h.model.FindDialog = ui.FindDialogState{
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
	h.model.FindDialog = ui.FindDialogState{}
}

func (h *Handler) findVolumeGate(st *ui.FindDialogState) diskusage.ListingVolumeGate {
	return diskusage.ListingVolumeGate{
		Enabled: st.StayOnCurrentVolume && st.ListingDeviceValid,
		RefDev:  st.ListingDevice,
		Valid:   st.ListingDeviceValid,
	}
}

func (h *Handler) findScopeRoots(st *ui.FindDialogState) []string {
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
	st := &h.model.FindDialog
	if !st.Open || st.RootPath == "" {
		return
	}
	st.IndexErr = ""
	st.Entries = nil
	st.IndexedCount = 0
	st.Ranked = nil
	st.MatchRanges = nil
	st.Selected = 0
	st.ListScroll = 0
	h.startFindIndexer()
	h.syncFindDialogRanks()
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

func (h *Handler) findSelectionSkipRoots(st *ui.FindDialogState) []string {
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
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(WakePayload{}))
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
		st.IndexDone = true
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

func (h *Handler) appendFindBatch(st *ui.FindDialogState, batch []findpkg.Entry) {
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
		st.Entries = append(st.Entries, ui.FindEntry{
			Path:    p,
			RelLine: findRelLine(st.RootPath, p),
			IsDir:   e.IsDir,
			Type:    e.Type,
		})
	}
	st.IndexedCount = len(st.Entries)
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
		if pathInFindSelectionScope(e.Path, st.SelectionDirRoots) {
			filtered = append(filtered, e)
			h.sessionMu.Lock()
			h.indexedPaths[filepath.Clean(e.Path)] = struct{}{}
			h.sessionMu.Unlock()
		}
	}
	st.Entries = filtered
	st.IndexedCount = len(st.Entries)
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
				h.syncFindDialogRanks()
				needRender = true
			default:
				goto drained
			}
		}
	}
drained:
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
	lines := make([]string, len(st.Entries))
	for i, e := range st.Entries {
		lines[i] = e.RelLine
	}
	q := search.Parse(st.Query)
	opts := search.Options{CaseInsensitive: h.config.CaseInsensitiveFilter}
	ranked := q.Rank(lines, opts)
	st.Ranked = make([]int, len(ranked))
	st.MatchRanges = make([][]search.Range, len(st.Entries))
	for i := range st.MatchRanges {
		st.MatchRanges[i] = nil
	}
	for i, r := range ranked {
		st.Ranked[i] = r.Index
		if r.Index >= 0 && r.Index < len(st.MatchRanges) {
			st.MatchRanges[r.Index] = r.Result.Ranges
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
	ui.EnsureFindListScroll(st, h.findDialogListRows())
}

func (h *Handler) findDialogListRows() int {
	termW, termH := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(termW, termH)
	checkboxRows := 1
	if h.model.FindDialog.ShowSearchSelectionsOption {
		checkboxRows = 2
	}
	baseHeight := 9 + checkboxRows
	listH := layout.Height - 4 - baseHeight
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	dialogHeight := baseHeight + listH
	if dialogHeight > layout.Height-2 {
		listH = layout.Height - 2 - baseHeight
		if listH < 4 {
			return 4
		}
	}
	return listH
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
	added := 0
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
		p.AddSelection(path)
		added++
	}
	h.model.ActivePanel = st.PanelID
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseDialog()
	if added == 0 {
		h.host.SetTransientMessage("No new selections", ui.MessageUrgencyInfo)
		return
	}
	h.host.SetTransientMessage(fmt.Sprintf("Added %d to selection", added), ui.MessageUrgencyInfo)
}

// NavigateFindCursor moves the active panel to the find dialog selection.
func (h *Handler) NavigateFindCursor() {
	st := &h.model.FindDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Entries) {
		return
	}
	ent := st.Entries[entIdx]
	path := filepath.Clean(ent.Path)
	panelID := st.PanelID

	if ent.IsDir {
		if err := h.host.NavigatePanelToDirectory(panelID, path, ""); err != nil {
			h.host.SetErrorMessage("Find", err)
			return
		}
	} else {
		dir := filepath.Clean(filepath.Dir(path))
		name := filepath.Base(path)
		if err := h.host.NavigatePanelToDirectory(panelID, dir, name); err != nil {
			h.host.SetErrorMessage("Find", err)
			return
		}
	}
	h.model.ActivePanel = panelID
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.host.PanelByID(panelID).EnsureCursorVisible(h.host.PanelViewportRows(panelID))
	h.CloseDialog()
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
	path := filepath.Clean(st.Entries[entIdx].Path)
	if path == "" {
		return
	}
	if st.MarkedPaths == nil {
		st.MarkedPaths = make(map[string]bool)
	}
	if st.MarkedPaths[path] {
		delete(st.MarkedPaths, path)
	} else {
		if clearFindMarkedConflicts(st, path, st.Entries[entIdx].IsDir) {
			h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
		st.MarkedPaths[path] = true
	}
	if st.Selected < len(st.Ranked)-1 {
		st.Selected++
		ui.EnsureFindListScroll(st, h.findDialogListRows())
	}
}

// ToggleStayOnVolume toggles stay-on-volume and restarts indexing when needed.
func (h *Handler) ToggleStayOnVolume() {
	st := &h.model.FindDialog
	st.StayOnCurrentVolume = !st.StayOnCurrentVolume
	h.restartFindIndexer()
}

func (h *Handler) HandleDialogKey(event *tcell.EventKey) {
	st := &h.model.FindDialog
	if id, ok := h.keys.Lookup(event); ok && id == keymap.ActionPanelSelectToggle {
		h.findDialogToggleSelectionAndAdvance()
		return
	}
	if ui.AltDialogOK(event) {
		h.ActivateDialogOK()
		return
	}
	if ui.AltDialogCancel(event) {
		h.CloseDialog()
		return
	}

	if st.Focus == 0 {
		onChange := func() {
			h.syncFindDialogRanks()
			st.Selected = 0
			ui.EnsureFindListScroll(st, h.findDialogListRows())
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
		case 1:
			h.ToggleStayOnVolume()
		case 2:
			if st.FindDialogHasSelectionsCheckbox() {
				h.ToggleSearchOnlySelections()
			} else {
				h.ActivateDialogOK()
			}
		default:
			h.ActivateDialogOK()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := ui.FindDialogNavFocusKey(st.Focus, st.FindDialogHasSelectionsCheckbox(), event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				ui.EnsureFindListScroll(st, h.findDialogListRows())
			}
			break
		}
		if st.Focus == 0 && len(st.Ranked) > 0 {
			switch event.Key() {
			case tcell.KeyUp:
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
				ui.EnsureFindListScroll(st, h.findDialogListRows())
			case tcell.KeyDown:
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
				ui.EnsureFindListScroll(st, h.findDialogListRows())
			}
		}
	case tcell.KeyHome:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = 0
			ui.EnsureFindListScroll(st, h.findDialogListRows())
		}
	case tcell.KeyEnd:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			ui.EnsureFindListScroll(st, h.findDialogListRows())
		}
	case tcell.KeyPgUp:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, h.findDialogListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -step)
			ui.EnsureFindListScroll(st, h.findDialogListRows())
		}
	case tcell.KeyPgDn:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, h.findDialogListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), step)
			ui.EnsureFindListScroll(st, h.findDialogListRows())
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
			if st.Focus == 1 {
				h.ToggleStayOnVolume()
			}
		case 's', 'S':
			if st.Focus == 2 && st.FindDialogHasSelectionsCheckbox() {
				h.ToggleSearchOnlySelections()
			}
		case 'o', 'O':
			h.ActivateDialogOK()
		case 'c', 'C':
			h.CloseDialog()
		case ' ':
			switch st.Focus {
			case 1:
				h.ToggleStayOnVolume()
			case 2:
				if st.FindDialogHasSelectionsCheckbox() {
					h.ToggleSearchOnlySelections()
				} else {
					h.ActivateDialogOK()
				}
			case 3:
				if st.FindDialogHasSelectionsCheckbox() {
					h.ActivateDialogOK()
				} else {
					h.CloseDialog()
				}
			case 4:
				if st.FindDialogHasSelectionsCheckbox() {
					h.CloseDialog()
				}
			}
		}
	}
}

// clearFindMarkedConflicts removes marked paths that conflict with adding path (dir or file).
// Returns true if any mark was removed.
func clearFindMarkedConflicts(st *ui.FindDialogState, path string, isDir bool) bool {
	if st.MarkedPaths == nil {
		return false
	}
	added := filepath.Clean(path)
	var removed bool
	if isDir {
		for p := range st.MarkedPaths {
			clean := filepath.Clean(p)
			if panel.IsStrictPathDescendant(added, clean) {
				delete(st.MarkedPaths, p)
				removed = true
			}
		}
		return removed
	}
	for p := range st.MarkedPaths {
		clean := filepath.Clean(p)
		if !findMarkedPathIsDir(st, clean) {
			continue
		}
		if panel.IsStrictPathDescendant(clean, added) {
			delete(st.MarkedPaths, p)
			removed = true
		}
	}
	return removed
}

func findMarkedPathIsDir(st *ui.FindDialogState, path string) bool {
	for _, e := range st.Entries {
		if filepath.Clean(e.Path) == path {
			return e.IsDir
		}
	}
	return false
}
