package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type quickViewFlushPayload struct {
	gen uint64
}

func (a *App) closeFilePreview() {
	a.commandsMu.Lock()
	a.model.FilePreview = ui.FilePreviewState{}
	a.commandsMu.Unlock()
	if a.model.ActiveSubFocus == ui.SubFocusInactivePreview {
		a.model.ActiveSubFocus = ui.SubFocusFileList
	}
}

func (a *App) filePreviewOpen() bool {
	a.commandsMu.RLock()
	defer a.commandsMu.RUnlock()
	return a.model.FilePreview.Open
}

// inactivePanelPreviewLayoutMetrics returns inner text width and preview body row count for the
// inactive column. filePreviewOpenForLayout should match whether preview is treated as open for
// panel zoom (true = even split when zoom would otherwise apply). ok is false when layout.TooSmall.
func (a *App) inactivePanelPreviewLayoutMetrics(filePreviewOpenForLayout bool) (textW, contentH int, ok bool) {
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSizePreview(w, h, filePreviewOpenForLayout)
	if lay.TooSmall {
		return 1, 0, false
	}
	inactiveID := a.inactivePanelID()
	col := lay.Left
	p := &a.model.Left
	if inactiveID == ui.RightPanel {
		col = lay.Right
		p = &a.model.Right
	}
	stripN := ui.SelectionsStripLayoutItemCount(p, inactiveID, a.model.ActivePanel, a.model.ThemeDialog.Open)
	fileCol, _ := ui.SplitPanelColumn(col, stripN, a.model.SelectionsPanelMaxRows, ui.MinFileListContentRows)
	tw := fileCol.Width - 4
	if tw < 1 {
		tw = 1
	}
	return tw, ui.JobsPanelContentRows(fileCol), true
}

func (a *App) filePreviewScrollMetrics() (textW, contentH, lineCount int) {
	tw, ch, layOK := a.inactivePanelPreviewLayoutMetrics(a.filePreviewOpen())
	if !layOK {
		return tw, ch, 0
	}
	textW, contentH = tw, ch
	a.commandsMu.RLock()
	t := a.model.FilePreview.CombinedText
	ph := a.model.FilePreview.Phase
	em := a.model.FilePreview.ErrorMsg
	a.commandsMu.RUnlock()
	switch ph {
	case ui.FilePreviewPhasePending, ui.FilePreviewPhaseRunning:
		lineCount = 1
	case ui.FilePreviewPhaseDone:
		if strings.TrimSpace(em) != "" {
			lineCount = 1
			break
		}
		lineCount = ui.FilePreviewTotalLines(t, textW)
		if lineCount < 1 {
			lineCount = 1
		}
	default:
		lineCount = 1
	}
	return textW, contentH, lineCount
}

func (a *App) clampFilePreviewScroll() {
	_, ch, lc := a.filePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchFilePreview(func(st *ui.FilePreviewState) {
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (a *App) previewScrollBy(delta int) {
	_, ch, lc := a.filePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll += delta
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (a *App) previewScrollTo(scroll int) {
	_, ch, lc := a.filePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll = scroll
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

// cycleSubFocusForTabWithPreview advances Tab focus among file list, selections strip (if any),
// and preview pane. Tab from preview returns to the active panel's file list (see tryDispatchFilePreviewFocus).
func (a *App) cycleSubFocusForTabWithPreview() {
	switch a.model.ActiveSubFocus {
	case ui.SubFocusFileList:
		if a.activePanel().SelectionsStripCount() > 0 {
			a.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
			a.activePanel().EnsureSelectionsStripCursorVisible(a.selectionsStripViewportRows(a.model.ActivePanel))
			return
		}
		a.model.ActiveSubFocus = ui.SubFocusInactivePreview
	case ui.SubFocusSelectionsStrip:
		a.model.ActiveSubFocus = ui.SubFocusInactivePreview
	case ui.SubFocusInactivePreview:
		// Defensive: Tab from preview is handled in tryDispatchFilePreviewFocus before dispatch reaches here.
		a.model.ActiveSubFocus = ui.SubFocusFileList
	}
}

// tryDispatchFilePreviewFocus handles keys while keyboard focus is on the inactive preview pane.
func (a *App) tryDispatchFilePreviewFocus(actionID string) bool {
	if a.model.ViewMode != ui.ViewBrowser || a.model.ActiveSubFocus != ui.SubFocusInactivePreview {
		return false
	}
	if !a.filePreviewOpen() {
		a.model.ActiveSubFocus = ui.SubFocusFileList
		return false
	}
	_, ch, _ := a.filePreviewScrollMetrics()
	step := ch
	if step < 1 {
		step = 1
	}
	switch actionID {
	case keymap.ActionNavUp:
		a.previewScrollBy(-1)
		return true
	case keymap.ActionNavDown:
		a.previewScrollBy(1)
		return true
	case keymap.ActionNavPageUp:
		a.previewScrollBy(-step)
		return true
	case keymap.ActionNavPageDown:
		a.previewScrollBy(step)
		return true
	case keymap.ActionNavTop:
		a.previewScrollTo(0)
		return true
	case keymap.ActionNavBottom:
		_, ch2, lc := a.filePreviewScrollMetrics()
		a.previewScrollTo(max(0, lc-ch2))
		return true
	case keymap.ActionPanelSwitch:
		if a.model.QuickViewEnabled {
			a.switchPanel()
		} else {
			a.model.ActiveSubFocus = ui.SubFocusFileList
		}
		return true
	case keymap.ActionNavOpen:
		return true
	case keymap.ActionPanelFocusSelections:
		if a.activePanel().SelectionsStripCount() > 0 {
			a.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
			a.activePanel().EnsureSelectionsStripCursorVisible(a.selectionsStripViewportRows(a.model.ActivePanel))
		} else {
			a.model.ActiveSubFocus = ui.SubFocusFileList
		}
		return true
	default:
		return false
	}
}

func (a *App) patchFilePreview(fn func(*ui.FilePreviewState)) {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	fn(&a.model.FilePreview)
}

func (a *App) patchFilePreviewTarget(fullscreen bool, fn func(*ui.FilePreviewState)) {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	if fullscreen {
		fn(&a.model.FullscreenFilePreview)
	} else {
		fn(&a.model.FilePreview)
	}
}

// handleQuickViewToggle toggles inactive-column quick view (Shift+F3 / Left or Right menu).
func (a *App) handleQuickViewToggle() {
	if a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() {
		return
	}
	if a.inQuickFilterUI() {
		return
	}
	active := a.model.ActivePanel
	if a.model.QuickViewEnabled && a.model.QuickViewPanel == active {
		a.model.QuickViewEnabled = false
		a.model.QuickViewPanel = -1
		a.clearQuickViewDebounce()
		a.quickViewLastFingerprint = ""
		a.closeFilePreview()
		a.clearQuickViewDirOverlay()
		a.setTransientMessage("Quick view off", ui.MessageUrgencyInfo)
		return
	}
	a.model.QuickViewEnabled = true
	a.model.QuickViewPanel = active
	if a.model.SyncFollowEnabled {
		a.model.SyncFollowEnabled = false
		a.clearPanelSyncFollowNavCoalesce()
		a.setTransientMessage("Quick view on — sync disabled", ui.MessageUrgencyWarn)
	} else {
		a.setTransientMessage("Quick view on", ui.MessageUrgencyInfo)
	}
	a.clearCarouselPreviewNavCoalesce()
	a.applyQuickViewPreviewImmediately()
}

// pauseQuickViewDisplay hides inactive-column preview while quick view stays latched on the driver panel.
func (a *App) pauseQuickViewDisplay() {
	a.clearQuickViewDebounce()
	a.quickViewLastFingerprint = ""
	a.closeFilePreview()
	a.clearQuickViewDirOverlay()
	if a.model.ActiveSubFocus == ui.SubFocusInactivePreview {
		a.model.ActiveSubFocus = ui.SubFocusFileList
	}
}

// resumeQuickViewDisplay restores inactive-column preview after returning to the quick-view driver panel.
func (a *App) resumeQuickViewDisplay() {
	a.clearCarouselPreviewNavCoalesce()
	a.applyQuickViewPreviewImmediately()
}

func (a *App) patchColumnPreviewMessage(titleBase, msg string) {
	a.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.Path = ""
		st.TitleBase = titleBase
		st.CombinedText = ""
		st.Scroll = 0
		st.ExitCode = 0
		st.ErrorMsg = msg
	})
	a.postCommandWake()
	a.clampFilePreviewScroll()
}

// quickViewFingerprint identifies the current quick-view highlight for debouncing.
func (a *App) quickViewFingerprint() string {
	path, _, mode := a.quickViewWantFile()
	switch mode {
	case quickViewWantNone:
		return "none"
	case quickViewWantFile:
		return "f:" + path
	case quickViewWantDir:
		if path, ok := a.syncFollowTargetPath(a.activePanel()); ok {
			return "d:" + path
		}
		return "d:none"
	case quickViewWantStatErr:
		return "e:stat"
	}
	return "none"
}

type quickViewWantMode int

const (
	quickViewWantNone quickViewWantMode = iota
	quickViewWantFile
	quickViewWantDir
	quickViewWantStatErr
)

// quickViewWantFile returns an absolute file path to preview when mode == quickViewWantFile.
func (a *App) quickViewWantFile() (path string, workDir string, mode quickViewWantMode) {
	p := a.activePanel()
	workDir = p.PathString()
	if a.model.ActiveSubFocus == ui.SubFocusSelectionsStrip && p.SelectionsStripCount() > 0 {
		selPath, ok := p.SelectedPathAtStripIndex(p.SelectionsStripCursor)
		if !ok {
			return "", workDir, quickViewWantNone
		}
		selPath = filepath.Clean(selPath)
		if selPath == "" || selPath == "." {
			return "", workDir, quickViewWantNone
		}
		fi, err := os.Stat(selPath)
		if err != nil {
			return "", workDir, quickViewWantStatErr
		}
		if fi.IsDir() {
			return "", workDir, quickViewWantDir
		}
		return selPath, workDir, quickViewWantFile
	}
	entry, ok := p.CurrentEntry()
	if !ok {
		return "", workDir, quickViewWantNone
	}
	if entry.Type == localfs.EntryDirectory {
		return "", workDir, quickViewWantDir
	}
	path = filepath.Clean(entry.Path)
	if path == "" || path == "." {
		return "", workDir, quickViewWantNone
	}
	return path, workDir, quickViewWantFile
}

func (a *App) clearQuickViewDirOverlay() {
	a.model.QuickViewDirOverlay = panel.State{}
	a.model.QuickViewDirOverlayActive = false
	a.model.QuickViewDirOverlayPanelID = -1
}

// populateQuickViewDirOverlay fills the inactive-column directory overlay. When driver or
// follower already lists the target directory, the live cursor is mirrored. Otherwise the
// listing is built with the same snapshot path as carousel child preview (history recall).
func (a *App) populateQuickViewDirOverlay(ov *panel.State, driver, follower *panel.State, dir string, panelID int) error {
	canonical := panel.CleanPathString(dir)
	if canonical == "" {
		return errors.New("empty directory path")
	}
	a.initQuickViewDirOverlayFromFollower(ov, driver, follower, panelID)
	if follower != nil && follower.ListingAtPath(canonical) {
		ov.CloneListingFrom(follower)
		return nil
	}
	if driver != nil && driver.ListingAtPath(canonical) {
		ov.CloneListingFrom(driver)
		return nil
	}
	vr := a.panelViewportRows(panelID)
	snap, err := driver.SnapshotDirectory(canonical, vr, driver, follower)
	if err != nil {
		return err
	}
	if err := ov.Load(canonical); err != nil {
		return err
	}
	ov.Path = snap.Path
	ov.Entries = snap.Entries
	ov.Cursor = snap.Cursor
	ov.ScrollOffset = snap.Scroll
	ov.EnsureCursorInViewport(vr)
	return nil
}

// initQuickViewDirOverlayFromFollower prepares QuickViewDirOverlay for a directory preview load.
// The real inactive panel path, cursor, and selection are not modified.
func (a *App) initQuickViewDirOverlayFromFollower(ov *panel.State, driver, follower *panel.State, followerID int) {
	*ov = panel.State{
		Sort:                       driver.Sort,
		Filter:                     driver.Filter,
		ShowHidden:                 driver.ShowHidden,
		ListFormat:                 driver.ListFormat,
		CenterScrolling:            driver.CenterScrolling,
		Gitignore:                  follower.Gitignore,
		DiskSorter:                 follower.DiskSorter,
		InDiskUsageScanScope:       follower.InDiskUsageScanScope,
		SuppressHeavyPathProbes:    follower.SuppressHeavyPathProbes,
		ScheduleRemoteLoad:         follower.ScheduleRemoteLoad,
		IdleDiskTotalsSort:         follower.IdleDiskTotalsSort,
		DiskUsageIdleSortActivated: follower.DiskUsageIdleSortActivated,
		HistoryCursorByPath:        panel.MergeHistoryCursorByPath(follower.HistoryCursorByPath, driver.HistoryCursorByPath),
	}
	ov.FileListViewportRows = func() int { return a.panelViewportRows(followerID) }
}

// quickViewFollowDirectory loads the highlighted directory into the inactive-column overlay
// (same target-path rules and Load semantics as latched panel sync; real panel unchanged).
func (a *App) quickViewFollowDirectory() {
	driver := a.activePanel()
	targetPath, ok := a.syncFollowTargetPath(driver)
	if !ok {
		a.clearQuickViewDirOverlay()
		a.patchColumnPreviewMessage("", "Quick view: select a folder")
		return
	}
	a.closeFilePreview()
	followerID := a.inactivePanelID()
	follower := a.panelByID(followerID)
	targetPath = panel.CleanPathString(targetPath)
	if targetPath == "" {
		return
	}
	if a.pathVolumeContendsWithActiveJob(targetPath) {
		return
	}
	ov := &a.model.QuickViewDirOverlay
	a.model.QuickViewDirOverlayPanelID = followerID
	if err := a.populateQuickViewDirOverlay(ov, driver, follower, targetPath, followerID); err != nil {
		return
	}
	a.model.QuickViewDirOverlayActive = true
}

func (a *App) applyQuickViewPreviewImmediately() {
	if !a.model.QuickViewDisplayActive() || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() || a.inQuickFilterUI() {
		return
	}
	a.applyQuickViewPreviewNow()
	a.quickViewLastFingerprint = a.quickViewFingerprint()
}

func (a *App) applyQuickViewPreviewNow() {
	if !a.model.QuickViewDisplayActive() || a.model.ViewMode != ui.ViewBrowser {
		return
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() || a.inQuickFilterUI() {
		return
	}
	path, workDir, mode := a.quickViewWantFile()
	switch mode {
	case quickViewWantNone:
		a.clearQuickViewDirOverlay()
		a.patchColumnPreviewMessage("", "Quick view: no file")
	case quickViewWantDir:
		a.quickViewFollowDirectory()
	case quickViewWantStatErr:
		a.clearQuickViewDirOverlay()
		a.patchColumnPreviewMessage("", "Quick view: cannot read selection")
	case quickViewWantFile:
		a.clearQuickViewDirOverlay()
		if err := localfs.CheckFilePreviewable(path); err != nil {
			switch {
			case errors.Is(err, localfs.ErrFilePreviewBinary):
				a.patchColumnPreviewMessage(filepath.Base(path), "Quick view: not a text file")
			case errors.Is(err, localfs.ErrFilePreviewIsDir):
				a.patchColumnPreviewMessage("", "Quick view: not a file")
			default:
				a.patchColumnPreviewMessage(filepath.Base(path), err.Error())
			}
			return
		}
		tw, _, layOK := a.inactivePanelPreviewLayoutMetrics(true)
		if !layOK {
			tw = 1
		}
		argv, err := cmdrun.BuildFilePreviewArgv(a.config.Preview.Command, path, tw)
		if err != nil {
			a.patchColumnPreviewMessage(filepath.Base(path), "Preview command: "+err.Error())
			return
		}
		titleBase := filepath.Base(path)
		a.patchFilePreview(func(st *ui.FilePreviewState) {
			st.Open = true
			st.Phase = ui.FilePreviewPhasePending
			st.Path = path
			st.TitleBase = titleBase
			st.CombinedText = ""
			st.Scroll = 0
			st.ExitCode = 0
			st.ErrorMsg = ""
		})
		a.postCommandWake()
		gen := a.filePreviewRunGen.Add(1)
		go a.runFilePreview(a.commandsCtx, path, argv, workDir, false, gen)
	}
}

func (a *App) clearQuickViewDebounce() {
	a.quickViewDebounceMu.Lock()
	if a.quickViewDebounceTimer != nil {
		if !a.quickViewDebounceTimer.Stop() {
			select {
			case <-a.quickViewDebounceTimer.C:
			default:
			}
		}
		a.quickViewDebounceTimer = nil
	}
	a.quickViewDebounceMu.Unlock()
	a.quickViewDebounceGen.Add(1)
	a.quickViewNavSkipReconcile.Store(false)
}

// quickViewNavCoalesceContext is true when file-list nav should coalesce quick view preview updates.
func (a *App) quickViewNavCoalesceContext() bool {
	return a.model.ViewMode == ui.ViewBrowser &&
		a.model.QuickViewDisplayActive() &&
		a.model.ActiveSubFocus == ui.SubFocusFileList &&
		!a.model.Menu.Open &&
		!a.model.ModalDialogOpen() &&
		!a.inQuickFilterUI()
}

// clearQuickViewNavCoalesce stops pending file-list nav coalesce and allows reconcile to preview again.
func (a *App) clearQuickViewNavCoalesce() {
	a.clearQuickViewDebounce()
}

func (a *App) scheduleQuickViewDebounceTimer(gen uint64) {
	delay := time.Duration(a.config.UI.QuickViewPreviewDebounceMS) * time.Millisecond
	a.quickViewDebounceMu.Lock()
	defer a.quickViewDebounceMu.Unlock()
	if a.quickViewDebounceTimer != nil {
		if !a.quickViewDebounceTimer.Stop() {
			select {
			case <-a.quickViewDebounceTimer.C:
			default:
			}
		}
		a.quickViewDebounceTimer = nil
	}
	a.quickViewDebounceTimer = time.AfterFunc(delay, func() {
		a.quickViewDebounceMu.Lock()
		a.quickViewDebounceTimer = nil
		a.quickViewDebounceMu.Unlock()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(quickViewFlushPayload{gen: gen}))
	})
}

func (a *App) armQuickViewNavCoalesceAfterListNav() {
	if a.config.UI.QuickViewPreviewDebounceMS <= 0 {
		return
	}
	if !a.quickViewNavCoalesceContext() {
		return
	}
	gen := a.quickViewDebounceGen.Add(1)
	a.quickViewNavSkipReconcile.Store(true)
	a.scheduleQuickViewDebounceTimer(gen)
}

func (a *App) armQuickViewPreviewDebounce() {
	if a.config.UI.QuickViewPreviewDebounceMS <= 0 {
		a.applyQuickViewPreviewNow()
		a.quickViewLastFingerprint = a.quickViewFingerprint()
		return
	}
	gen := a.quickViewDebounceGen.Add(1)
	a.scheduleQuickViewDebounceTimer(gen)
}

func (a *App) applyQuickViewPreviewFlush(p quickViewFlushPayload) bool {
	if p.gen != a.quickViewDebounceGen.Load() {
		return false
	}
	a.quickViewNavSkipReconcile.Store(false)
	if !a.model.QuickViewDisplayActive() || a.model.ViewMode != ui.ViewBrowser {
		return false
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() || a.inQuickFilterUI() {
		return false
	}
	a.applyQuickViewPreviewNow()
	a.quickViewLastFingerprint = a.quickViewFingerprint()
	return true
}

func (a *App) reconcileQuickViewPreview() {
	if !a.model.QuickViewDisplayActive() || a.model.ViewMode != ui.ViewBrowser {
		a.clearQuickViewDebounce()
		return
	}
	if a.quickViewNavSkipReconcile.Load() {
		return
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() || a.inQuickFilterUI() {
		return
	}
	sig := a.quickViewFingerprint()
	if sig == a.quickViewLastFingerprint {
		return
	}
	a.armQuickViewPreviewDebounce()
}

func (a *App) runFilePreview(ctx context.Context, path string, argv []string, workDir string, fullscreen bool, runGen uint64) {
	if runGen != a.filePreviewRunGen.Load() {
		return
	}
	runningApplied := false
	a.patchFilePreviewTarget(fullscreen, func(st *ui.FilePreviewState) {
		if !st.Open || st.Path != path {
			return
		}
		st.Phase = ui.FilePreviewPhaseRunning
		runningApplied = true
	})
	if runningApplied && runGen == a.filePreviewRunGen.Load() {
		a.postCommandWake()
	}

	select {
	case <-ctx.Done():
		if runGen != a.filePreviewRunGen.Load() {
			return
		}
		canceledApplied := false
		a.patchFilePreviewTarget(fullscreen, func(st *ui.FilePreviewState) {
			if st.Path != path {
				return
			}
			st.Phase = ui.FilePreviewPhaseDone
			st.ErrorMsg = "Canceled"
			canceledApplied = true
		})
		if canceledApplied && runGen == a.filePreviewRunGen.Load() {
			a.postCommandWake()
			if fullscreen {
				a.clampFullscreenFilePreviewScroll()
			} else {
				a.clampFilePreviewScroll()
			}
		}
		return
	default:
	}

	res := cmdrun.Run(ctx, argv, workDir, cmdrun.MaxStreamBytes)

	if runGen != a.filePreviewRunGen.Load() {
		return
	}
	doneApplied := false
	a.patchFilePreviewTarget(fullscreen, func(st *ui.FilePreviewState) {
		if !st.Open || st.Path != path {
			return
		}
		st.Phase = ui.FilePreviewPhaseDone
		if res.LaunchErr != nil {
			st.ErrorMsg = res.LaunchErr.Error()
			st.ExitCode = -1
			doneApplied = true
			return
		}
		st.ExitCode = res.ExitCode
		var b strings.Builder
		b.WriteString(string(res.Stdout))
		if len(res.Stderr) > 0 {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString("--- stderr ---\n")
			b.WriteString(string(res.Stderr))
		}
		st.CombinedText = b.String()
		if res.StdoutTrim || res.StderrTrim {
			st.CombinedText += "\n\n[output truncated]\n"
		}
		doneApplied = true
	})
	if doneApplied && runGen == a.filePreviewRunGen.Load() {
		a.postCommandWake()
		if fullscreen {
			a.clampFullscreenFilePreviewScroll()
		} else {
			a.clampFilePreviewScroll()
		}
	}
}
