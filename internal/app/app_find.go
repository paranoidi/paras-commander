package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/diskusage"
	"github.com/paranoidi/paras-commander/internal/find"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type findWakePayload struct {
	finished bool
}

func (a *App) openFindDialog(panelID int) {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
	if a.model.FindDialog.Open {
		a.closeFindDialog()
	}
	p := a.panelByID(panelID)
	root := filepath.Clean(p.Path)
	selRoots := panel.PruneNestedPaths(p.SelectedDirectoryPaths())
	a.model.FindDialog = ui.FindDialogState{
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
	a.startFindIndexer()
}

func (a *App) closeFindDialog() {
	a.stopFindIndexer()
	a.model.FindDialog = ui.FindDialogState{}
}

func (a *App) findVolumeGate(st *ui.FindDialogState) diskusage.ListingVolumeGate {
	return diskusage.ListingVolumeGate{
		Enabled: st.StayOnCurrentVolume && st.ListingDeviceValid,
		RefDev:  st.ListingDevice,
		Valid:   st.ListingDeviceValid,
	}
}

func (a *App) findScopeRoots(st *ui.FindDialogState) []string {
	if st.ShowSearchSelectionsOption && st.SearchOnlySelections && len(st.SelectionDirRoots) > 0 {
		out := make([]string, len(st.SelectionDirRoots))
		for i, r := range st.SelectionDirRoots {
			out[i] = filepath.Clean(r)
		}
		return out
	}
	return []string{filepath.Clean(st.RootPath)}
}

func (a *App) startFindIndexer() {
	st := &a.model.FindDialog
	if !st.Open || st.RootPath == "" {
		return
	}
	a.findSessionMu.Lock()
	a.findBatchCh = make(chan []find.Entry, 32)
	a.findWalks = make(map[string]*findWalk)
	a.findIndexedPaths = make(map[string]struct{})
	a.findCompletedRoots = make(map[string]struct{})
	a.findSessionMu.Unlock()

	st.Indexing = true
	st.IndexDone = false
	st.IndexErr = ""
	for _, root := range a.findScopeRoots(st) {
		a.startFindWalk(root, false)
	}
}

func (a *App) restartFindIndexer() {
	a.stopFindIndexer()
	st := &a.model.FindDialog
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
	a.startFindIndexer()
	a.syncFindDialogRanks()
}

func (a *App) stopFindIndexer() {
	a.findSessionMu.Lock()
	walks := a.findWalks
	ch := a.findBatchCh
	a.findWalks = nil
	a.findBatchCh = nil
	a.findIndexedPaths = nil
	a.findCompletedRoots = nil
	a.findSessionMu.Unlock()
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

func (a *App) findWalkActive(root string) bool {
	a.findSessionMu.Lock()
	defer a.findSessionMu.Unlock()
	_, ok := a.findWalks[filepath.Clean(root)]
	return ok
}

func (a *App) findWalkCompleted(root string) bool {
	a.findSessionMu.Lock()
	defer a.findSessionMu.Unlock()
	_, ok := a.findCompletedRoots[filepath.Clean(root)]
	return ok
}

func (a *App) findSelectionSkipRoots(st *ui.FindDialogState) []string {
	if !st.ShowSearchSelectionsOption {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	a.findSessionMu.Lock()
	for root := range a.findWalks {
		r := filepath.Clean(root)
		if r == filepath.Clean(st.RootPath) {
			continue
		}
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	for root := range a.findCompletedRoots {
		r := filepath.Clean(root)
		if r == filepath.Clean(st.RootPath) {
			continue
		}
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	a.findSessionMu.Unlock()
	for _, r := range st.SelectionDirRoots {
		r = filepath.Clean(r)
		if _, ok := seen[r]; !ok {
			seen[r] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

func (a *App) startFindWalk(root string, skipIndexedSelectionSubtrees bool) {
	st := &a.model.FindDialog
	root = filepath.Clean(root)
	if root == "" {
		return
	}
	a.findSessionMu.Lock()
	if a.findWalks == nil {
		a.findWalks = make(map[string]*findWalk)
	}
	if _, exists := a.findWalks[root]; exists {
		a.findSessionMu.Unlock()
		return
	}
	if _, done := a.findCompletedRoots[root]; done {
		a.findSessionMu.Unlock()
		return
	}
	ch := a.findBatchCh
	a.findSessionMu.Unlock()
	if ch == nil {
		return
	}

	volumeSkip := diskusage.ComposeListingVolumeIgnore(nil, a.findVolumeGate(st))
	var shouldSkip func(string) bool
	if skipIndexedSelectionSubtrees {
		skipRoots := a.findSelectionSkipRoots(st)
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

	sess := find.Start(context.Background(), root, find.Options{
		ShowHidden:    st.ShowHidden,
		ShouldSkipDir: shouldSkip,
	})
	a.findSessionMu.Lock()
	if a.findBatchCh != ch {
		a.findSessionMu.Unlock()
		sess.Close()
		return
	}
	a.findWalks[root] = &findWalk{root: root, sess: sess}
	st.Indexing = true
	st.IndexDone = false
	a.findSessionMu.Unlock()
	go a.readFindSession(sess, ch, root)
}

func (a *App) readFindSession(sess *find.Session, ch chan []find.Entry, root string) {
	for batch := range sess.Results() {
		a.findSessionMu.Lock()
		active := a.findBatchCh == ch
		a.findSessionMu.Unlock()
		if !active {
			return
		}
		select {
		case ch <- batch:
		default:
			ch <- batch
		}
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(findWakePayload{}))
	}
	<-sess.Done()
	err := sess.Err()
	a.findSessionMu.Lock()
	if a.findWalks != nil {
		delete(a.findWalks, root)
	}
	if a.findCompletedRoots == nil {
		a.findCompletedRoots = make(map[string]struct{})
	}
	a.findCompletedRoots[root] = struct{}{}
	a.findSessionMu.Unlock()
	st := &a.model.FindDialog
	if st.Open && err != nil {
		st.IndexErr = err.Error()
	}
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(findWakePayload{finished: true}))
}

func (a *App) findAllWalksDone() bool {
	a.findSessionMu.Lock()
	defer a.findSessionMu.Unlock()
	return len(a.findWalks) == 0
}

func (a *App) updateFindIndexingState() {
	st := &a.model.FindDialog
	if !st.Open {
		return
	}
	if a.findAllWalksDone() {
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

func (a *App) appendFindBatch(st *ui.FindDialogState, batch []find.Entry) {
	if len(batch) == 0 {
		return
	}
	a.findSessionMu.Lock()
	if a.findIndexedPaths == nil {
		a.findIndexedPaths = make(map[string]struct{})
	}
	a.findSessionMu.Unlock()
	for _, e := range batch {
		p := filepath.Clean(e.Path)
		if p == "" {
			continue
		}
		a.findSessionMu.Lock()
		if _, dup := a.findIndexedPaths[p]; dup {
			a.findSessionMu.Unlock()
			continue
		}
		a.findIndexedPaths[p] = struct{}{}
		a.findSessionMu.Unlock()
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

func (a *App) filterFindEntriesToSelectionScope() {
	st := &a.model.FindDialog
	if len(st.SelectionDirRoots) == 0 {
		return
	}
	filtered := st.Entries[:0]
	a.findSessionMu.Lock()
	a.findIndexedPaths = make(map[string]struct{})
	a.findSessionMu.Unlock()
	for _, e := range st.Entries {
		if pathInFindSelectionScope(e.Path, st.SelectionDirRoots) {
			filtered = append(filtered, e)
			a.findSessionMu.Lock()
			a.findIndexedPaths[filepath.Clean(e.Path)] = struct{}{}
			a.findSessionMu.Unlock()
		}
	}
	st.Entries = filtered
	st.IndexedCount = len(st.Entries)
	a.syncFindDialogRanks()
}

func (a *App) stopFindWalksOutsideSelectionScope() {
	st := &a.model.FindDialog
	panelRoot := filepath.Clean(st.RootPath)
	allowed := make(map[string]struct{}, len(st.SelectionDirRoots))
	for _, r := range st.SelectionDirRoots {
		allowed[filepath.Clean(r)] = struct{}{}
	}
	a.findSessionMu.Lock()
	var stop []*findWalk
	for root, w := range a.findWalks {
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
		delete(a.findWalks, w.root)
		delete(a.findCompletedRoots, w.root)
	}
	a.findSessionMu.Unlock()
	for _, w := range stop {
		if w.sess != nil {
			w.sess.Close()
		}
	}
}

func (a *App) widenFindIndexer() {
	st := &a.model.FindDialog
	panelRoot := filepath.Clean(st.RootPath)
	if a.findWalkActive(panelRoot) || a.findWalkCompleted(panelRoot) {
		a.updateFindIndexingState()
		return
	}
	a.startFindWalk(panelRoot, true)
	a.updateFindIndexingState()
}

func (a *App) narrowFindIndexer() {
	a.stopFindWalksOutsideSelectionScope()
	a.filterFindEntriesToSelectionScope()
	a.updateFindIndexingState()
}

func (a *App) toggleFindSearchOnlySelections() {
	st := &a.model.FindDialog
	if !st.ShowSearchSelectionsOption {
		return
	}
	st.SearchOnlySelections = !st.SearchOnlySelections
	if st.SearchOnlySelections {
		a.narrowFindIndexer()
	} else {
		a.widenFindIndexer()
	}
}

func (a *App) pollFindUpdates(payload findWakePayload) bool {
	needRender := false
	a.findSessionMu.Lock()
	ch := a.findBatchCh
	a.findSessionMu.Unlock()
	if ch != nil {
		for {
			select {
			case batch, ok := <-ch:
				if !ok {
					goto drained
				}
				st := &a.model.FindDialog
				if !st.Open {
					continue
				}
				a.appendFindBatch(st, batch)
				a.syncFindDialogRanks()
				needRender = true
			default:
				goto drained
			}
		}
	}
drained:
	st := &a.model.FindDialog
	if st.Open && (st.Indexing || payload.finished) {
		if a.findAllWalksDone() {
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

func (a *App) syncFindDialogRanks() {
	st := &a.model.FindDialog
	if !st.Open {
		return
	}
	lines := make([]string, len(st.Entries))
	for i, e := range st.Entries {
		lines[i] = e.RelLine
	}
	q := search.Parse(st.Query)
	opts := search.Options{CaseInsensitive: a.config.CaseInsensitiveFilter}
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
	ui.EnsureFindListScroll(st, a.findDialogListRows())
}

func (a *App) findDialogListRows() int {
	termW, termH := a.screen.Size()
	layout := a.layoutForTerminalSize(termW, termH)
	checkboxRows := 1
	if a.model.FindDialog.ShowSearchSelectionsOption {
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

func (a *App) activateFindDialogOK() {
	if a.findDialogMarkedCount() > 0 {
		a.applyFindDialogMarkedSelections()
		return
	}
	a.navigateFindCursor()
}

func (a *App) findDialogMarkedCount() int {
	st := &a.model.FindDialog
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

func (a *App) applyFindDialogMarkedSelections() {
	st := &a.model.FindDialog
	p := a.panelByID(st.PanelID)
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
	a.model.ActivePanel = st.PanelID
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.closeFindDialog()
	if added == 0 {
		a.setTransientMessage("No new selections", ui.MessageUrgencyInfo)
		return
	}
	a.setTransientMessage(fmt.Sprintf("Added %d to selection", added), ui.MessageUrgencyInfo)
}

func (a *App) navigateFindCursor() {
	st := &a.model.FindDialog
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
		if err := a.navigatePanelToDirectory(panelID, path, ""); err != nil {
			a.setErrorMessage("Find", err)
			return
		}
	} else {
		dir := filepath.Clean(filepath.Dir(path))
		name := filepath.Base(path)
		if err := a.navigatePanelToDirectory(panelID, dir, name); err != nil {
			a.setErrorMessage("Find", err)
			return
		}
	}
	a.model.ActivePanel = panelID
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.panelByID(panelID).EnsureCursorVisible(a.panelViewportRows(panelID))
	a.closeFindDialog()
}

func (a *App) findDialogToggleSelectionAndAdvance() {
	st := &a.model.FindDialog
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
			a.setTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		}
		st.MarkedPaths[path] = true
	}
	if st.Selected < len(st.Ranked)-1 {
		st.Selected++
		ui.EnsureFindListScroll(st, a.findDialogListRows())
	}
}

func (a *App) toggleFindStayOnVolume() {
	st := &a.model.FindDialog
	st.StayOnCurrentVolume = !st.StayOnCurrentVolume
	a.restartFindIndexer()
}

func (a *App) handleFindDialogKey(event *tcell.EventKey) {
	st := &a.model.FindDialog
	if id, ok := a.keys.Lookup(event); ok && id == keymap.ActionPanelSelectToggle {
		a.findDialogToggleSelectionAndAdvance()
		return
	}
	if ui.AltDialogOK(event) {
		a.activateFindDialogOK()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeFindDialog()
		return
	}

	if st.Focus == 0 {
		onChange := func() {
			a.syncFindDialogRanks()
			st.Selected = 0
			ui.EnsureFindListScroll(st, a.findDialogListRows())
		}
		if a.handleScrollingQueryKey(event, true, findDialogScrollingQuery(st, a.findDialogQueryWidth(), onChange)) {
			return
		}
	}

	switch event.Key() {
	case tcell.KeyInsert:
		a.findDialogToggleSelectionAndAdvance()
	case tcell.KeyEsc:
		a.closeFindDialog()
	case tcell.KeyEnter:
		switch st.Focus {
		case st.FindDialogCancelFocus():
			a.closeFindDialog()
		case 1:
			a.toggleFindStayOnVolume()
		case 2:
			if st.FindDialogHasSelectionsCheckbox() {
				a.toggleFindSearchOnlySelections()
			} else {
				a.activateFindDialogOK()
			}
		default:
			a.activateFindDialogOK()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := ui.FindDialogNavFocusKey(st.Focus, st.FindDialogHasSelectionsCheckbox(), event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				ui.EnsureFindListScroll(st, a.findDialogListRows())
			}
			break
		}
		if st.Focus == 0 && len(st.Ranked) > 0 {
			switch event.Key() {
			case tcell.KeyUp:
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
				ui.EnsureFindListScroll(st, a.findDialogListRows())
			case tcell.KeyDown:
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
				ui.EnsureFindListScroll(st, a.findDialogListRows())
			}
		}
	case tcell.KeyHome:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = 0
			ui.EnsureFindListScroll(st, a.findDialogListRows())
		}
	case tcell.KeyEnd:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			ui.EnsureFindListScroll(st, a.findDialogListRows())
		}
	case tcell.KeyPgUp:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.findDialogListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -step)
			ui.EnsureFindListScroll(st, a.findDialogListRows())
		}
	case tcell.KeyPgDn:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.findDialogListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), step)
			ui.EnsureFindListScroll(st, a.findDialogListRows())
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
				a.toggleFindStayOnVolume()
			}
		case 's', 'S':
			if st.Focus == 2 && st.FindDialogHasSelectionsCheckbox() {
				a.toggleFindSearchOnlySelections()
			}
		case 'o', 'O':
			a.activateFindDialogOK()
		case 'c', 'C':
			a.closeFindDialog()
		case ' ':
			switch st.Focus {
			case 1:
				a.toggleFindStayOnVolume()
			case 2:
				if st.FindDialogHasSelectionsCheckbox() {
					a.toggleFindSearchOnlySelections()
				} else {
					a.activateFindDialogOK()
				}
			case 3:
				if st.FindDialogHasSelectionsCheckbox() {
					a.activateFindDialogOK()
				} else {
					a.closeFindDialog()
				}
			case 4:
				if st.FindDialogHasSelectionsCheckbox() {
					a.closeFindDialog()
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
