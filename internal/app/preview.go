package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
)

type quickViewFlushPayload struct {
	gen uint64
}

type previewTarget int

const (
	previewTargetInactive previewTarget = iota
	previewTargetFullscreen
	previewTargetCarousel
)

func (a *App) patchPreviewState(target previewTarget, fn func(*ui.FilePreviewState)) {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	switch target {
	case previewTargetFullscreen:
		fn(&a.model.FullscreenFilePreview)
	case previewTargetCarousel:
		fn(&a.model.CarouselFilePreview)
	default:
		fn(&a.model.FilePreview)
	}
}

func (a *App) previewRunGenFor(target previewTarget) *atomic.Uint64 {
	switch target {
	case previewTargetCarousel:
		return &a.carouselFilePreviewRunGen
	default:
		return &a.filePreviewRunGen
	}
}

func (a *App) closeFilePreview() {
	a.commandsMu.Lock()
	a.model.FilePreview = ui.FilePreviewState{}
	a.commandsMu.Unlock()
	a.clearFilePreviewHold(previewTargetInactive)
	a.clearQuickViewDirOverlayVisualHold()
	if a.model.ActiveSubFocus == ui.SubFocusInactivePreview {
		a.model.ActiveSubFocus = ui.SubFocusFileList
	}
}

func (a *App) clearQuickViewDirOverlayVisualHold() {
	a.model.QuickViewDirOverlayVisualHold = false
	a.model.QuickViewDirOverlayVisualHoldPanel = panel.State{}
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
	col := lay.Primary
	p := &a.model.Primary
	if inactiveID == ui.SecondaryPanel {
		col = lay.Secondary
		p = &a.model.Secondary
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
		lineCount = a.filePreviewLineCount(textW)
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

// quickViewFilePreviewScrollable reports whether quick view is painting scrollable file preview text.
func (a *App) quickViewFilePreviewScrollable() bool {
	if !a.model.QuickViewDisplayActive() || a.model.QuickViewDirOverlayActive {
		return false
	}
	a.commandsMu.RLock()
	defer a.commandsMu.RUnlock()
	st := a.model.FilePreview
	if !st.Open || st.Phase != ui.FilePreviewPhaseDone {
		return false
	}
	if strings.TrimSpace(st.ErrorMsg) != "" {
		return false
	}
	if st.Source == ui.PreviewSourceInternalHighlighted {
		return len(st.HighlightedCells) > 0
	}
	return strings.TrimSpace(st.CombinedText) != ""
}

// tryDispatchQuickViewPreviewScroll handles Ctrl+J/Ctrl+K preview page keys.
// Scrolls carousel child preview or inactive quick-view preview when available;
// otherwise pages the active file list while quick view is latched.
func (a *App) tryDispatchQuickViewPreviewScroll(actionID string) bool {
	var pageDir int
	switch actionID {
	case keymap.ActionFileQuickViewPreviewPageUp:
		pageDir = -1
	case keymap.ActionFileQuickViewPreviewPageDown:
		pageDir = 1
	default:
		return false
	}
	if a.model.ViewMode != ui.ViewBrowser {
		return false
	}
	if a.carouselFilePreviewScrollable() {
		_, ch, _ := a.carouselFilePreviewScrollMetrics()
		step := ch
		if step < 1 {
			step = 1
		}
		a.carouselPreviewScrollBy(pageDir * step)
		return true
	}
	if !a.model.QuickViewEnabled {
		return false
	}
	if a.quickViewFilePreviewScrollable() {
		_, ch, _ := a.filePreviewScrollMetrics()
		step := ch
		if step < 1 {
			step = 1
		}
		a.previewScrollBy(pageDir * step)
		return true
	}
	viewportRows := a.activeViewportRows()
	a.ensureCarouselChildCacheBeforeListNav()
	a.beginCarouselPreviewNavCoalesce()
	a.activePanel().Page(pageDir, viewportRows)
	a.armPanelSyncFollowNavCoalesceAfterListNav()
	a.armQuickViewNavCoalesceAfterListNav()
	a.armCarouselPreviewNavCoalesceAfterListNav()
	a.armCursorNameHintNavCoalesceAfterListNav()
	return true
}

// tryDispatchFileView handles file-preview/quick-view/diff-hunk-navigation actions.
func (a *App) tryDispatchFileView(actionID string) bool {
	switch actionID {
	case keymap.ActionFileView:
		a.openFilePreviewFullscreen()
	case keymap.ActionFileViewThemePicker:
		a.toggleFilePreviewThemePicker()
	case keymap.ActionFileViewToggleRaw:
		a.toggleFilePreviewRawMarkdown()
	case keymap.ActionFileViewDiffNextHunk:
		a.hunkNavigate(previewTargetInactive, 1)
	case keymap.ActionFileViewDiffPrevHunk:
		a.hunkNavigate(previewTargetInactive, -1)
	case keymap.ActionFileQuickView:
		a.handleQuickViewToggle()
	case keymap.ActionFileEdit:
		if a.model.ViewMode == ui.ViewFilePreview && a.model.FullscreenFilePreview.Open {
			a.editFullscreenPreviewFile()
		} else {
			a.editActiveFile()
		}
	default:
		return false
	}
	return true
}

func (a *App) patchFilePreview(fn func(*ui.FilePreviewState)) {
	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	fn(&a.model.FilePreview)
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
	a.closeCarouselFilePreview()
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
	a.closeCarouselFilePreview()
	a.applyQuickViewPreviewImmediately()
}

func (a *App) patchColumnPreviewMessage(titleBase, msg string) {
	a.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.Path = ""
		st.TitleBase = titleBase
		st.CombinedText = ""
		st.SetHighlightedCells(nil)
		st.Source = ui.PreviewSourceExternalANSI
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
		ScrollMode:                 driver.ScrollMode,
		ScrollEdgeMargin:           driver.ScrollEdgeMargin,
		Gitignore:                  follower.Gitignore,
		DiskSorter:                 follower.DiskSorter,
		SuppressHeavyPathProbes:    follower.SuppressHeavyPathProbes,
		ScheduleRemoteLoad:         follower.ScheduleRemoteLoad,
		IdleDiskTotalsSort:         follower.IdleDiskTotalsSort,
		DiskUsageIdleSortEligible:  follower.DiskUsageIdleSortEligible,
		DiskUsageIdleSortActivated: follower.DiskUsageIdleSortActivated,
		HistoryCursorByPath:        panel.MergeHistoryCursorByPath(follower.HistoryCursorByPath, driver.HistoryCursorByPath),
		ScheduleGitStatus:          a.quickViewGitStatusScheduler(),
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
	if a.model.Menu.Open || a.model.ModalDialogOpen() {
		return
	}
	path, workDir, mode := a.quickViewWantFile()
	switch mode {
	case quickViewWantNone:
		a.clearQuickViewDirOverlay()
		a.clearQuickViewDirOverlayVisualHold()
		a.patchColumnPreviewMessage("", "Quick view: no file")
	case quickViewWantDir:
		a.quickViewFollowDirectory()
	case quickViewWantStatErr:
		a.clearQuickViewDirOverlay()
		a.clearQuickViewDirOverlayVisualHold()
		a.patchColumnPreviewMessage("", "Quick view: cannot read selection")
	case quickViewWantFile:
		// When coming from a dir overlay, keep showing it until file content arrives.
		if a.model.QuickViewDirOverlayActive {
			a.model.QuickViewDirOverlayVisualHoldPanel = a.model.QuickViewDirOverlay
			a.model.QuickViewDirOverlayVisualHold = true
		}
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
		titleBase := filepath.Base(path)
		a.captureFilePreviewHold(previewTargetInactive)
		a.patchFilePreview(func(st *ui.FilePreviewState) {
			st.Open = true
			st.Phase = ui.FilePreviewPhasePending
			st.Path = path
			st.TitleBase = titleBase
			st.CombinedText = ""
			st.SetHighlightedCells(nil)
			st.Source = ui.PreviewSourceExternalANSI
			st.Scroll = 0
			st.ExitCode = 0
			st.ErrorMsg = ""
			st.IsDiff = false
			st.DiffHunkLines = nil
			st.GitStatusText = ""
			st.GitStatusThemeKey = ""
		})
		a.postCommandWake()
		gen := a.filePreviewRunGen.Add(1)
		go a.runPreview(a.commandsCtx, a.previewRequest(path, tw, workDir, a.inactivePreviewChromeBlocked(), a.gitStatusForPath(path), previewTargetInactive), previewTargetInactive, gen)
	}
}

// refreshInactiveFilePreview re-runs the current inactive-column (quick view) preview at its
// current path, e.g. after a terminal resize changes the inactive column's text width. Scroll is
// left untouched (unlike applyQuickViewPreviewNow, which is for opening/switching files).
func (a *App) refreshInactiveFilePreview() {
	a.commandsMu.RLock()
	st := a.model.FilePreview
	a.commandsMu.RUnlock()
	if !st.Open || st.Path == "" {
		return
	}
	tw, _, ok := a.inactivePanelPreviewLayoutMetrics(true)
	if !ok {
		return
	}
	workDir := a.activePanel().PathString()
	req := a.previewRequest(st.Path, tw, workDir, a.inactivePreviewChromeBlocked(), a.gitStatusForPath(st.Path), previewTargetInactive)
	gen := a.filePreviewRunGen.Add(1)
	a.postCommandWake()
	go a.runPreview(a.commandsCtx, req, previewTargetInactive, gen)
}

// refreshPreviewsAfterResize re-runs any open preview target whose current computed text width
// differs from the width its content was last requested at. Markdown word-wrap and table layout
// (internal/preview/mdformat) are baked into the emitted cells at request time, so a plain re-wrap
// at the new width (the downstream character-wrap cache) is not enough after a terminal resize.
func (a *App) refreshPreviewsAfterResize() {
	a.refreshPreviewTargetAfterResize(previewTargetInactive)
	a.refreshPreviewTargetAfterResize(previewTargetFullscreen)
	a.refreshPreviewTargetAfterResize(previewTargetCarousel)
}

func (a *App) refreshPreviewTargetAfterResize(target previewTarget) {
	a.commandsMu.RLock()
	var open bool
	switch target {
	case previewTargetFullscreen:
		open = a.model.FullscreenFilePreview.Open
	case previewTargetCarousel:
		open = a.model.CarouselFilePreview.Open
	default:
		open = a.model.FilePreview.Open
	}
	a.commandsMu.RUnlock()
	if !open {
		return
	}

	var tw int
	var ok bool
	switch target {
	case previewTargetFullscreen:
		tw, ok = a.fullscreenPreviewTextWidth()
	case previewTargetCarousel:
		tw, _, ok = a.carouselChildPreviewLayoutMetrics()
	default:
		tw, _, ok = a.inactivePanelPreviewLayoutMetrics(true)
	}
	if !ok || tw == a.previewLastWidth[target] {
		return
	}

	switch target {
	case previewTargetFullscreen:
		a.refreshFullscreenFilePreview()
	case previewTargetCarousel:
		a.refreshCarouselFilePreview()
	default:
		a.refreshInactiveFilePreview()
	}
}

func (a *App) clearQuickViewDebounce() {
	a.quickViewDebounce.Clear()
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
	delay := time.Duration(a.config.UI.KeyRepeatDebounceMS) * time.Millisecond
	a.quickViewDebounce.Reset(delay, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(quickViewFlushPayload{gen: gen}))
	})
}

func (a *App) armQuickViewNavCoalesceAfterListNav() {
	if a.config.UI.KeyRepeatDebounceMS <= 0 {
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
	if a.config.UI.KeyRepeatDebounceMS <= 0 {
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
	if a.model.Menu.Open || a.model.ModalDialogOpen() {
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
	if a.model.Menu.Open || a.model.ModalDialogOpen() {
		return
	}
	sig := a.quickViewFingerprint()
	if sig == a.quickViewLastFingerprint {
		return
	}
	a.armQuickViewPreviewDebounce()
}

func (a *App) previewRequest(path string, textW int, workDir string, chromeBlocked bool, gitStatus *gitstatus.Cell, target previewTarget) preview.Request {
	a.previewLastWidth[target] = textW
	req := preview.Request{
		Path:      path,
		TextWidth: textW,
		WorkDir:   workDir,
		Preview:   a.config.Preview,
		BaseStyle: ui.FilePreviewBodyStyle(a.styles, chromeBlocked),
	}
	if gitStatus != nil {
		req.GitDiff = true
		req.GitStatus = gitStatus
	}
	return req
}

// gitStatusForPath returns the git status for path from the active panel, or nil if unavailable.
func (a *App) gitStatusForPath(path string) *gitstatus.Cell {
	p := a.activePanel()
	if p == nil || p.GitByPath == nil {
		return nil
	}
	cell, ok := p.GitByPath[path]
	if !ok {
		return nil
	}
	cellCopy := cell
	return &cellCopy
}

func (a *App) runPreview(ctx context.Context, req preview.Request, target previewTarget, runGen uint64) {
	gen := a.previewRunGenFor(target)
	path := req.Path
	if runGen != gen.Load() {
		return
	}
	runningApplied := false
	a.patchPreviewState(target, func(st *ui.FilePreviewState) {
		if !st.Open || st.Path != path {
			return
		}
		st.Phase = ui.FilePreviewPhaseRunning
		runningApplied = true
	})
	if runningApplied && runGen == gen.Load() {
		a.postCommandWake()
	}

	select {
	case <-ctx.Done():
		if runGen != gen.Load() {
			return
		}
		canceledApplied := false
		a.patchPreviewState(target, func(st *ui.FilePreviewState) {
			if st.Path != path {
				return
			}
			st.Phase = ui.FilePreviewPhaseDone
			st.ErrorMsg = "Canceled"
			canceledApplied = true
		})
		if canceledApplied && runGen == gen.Load() {
			a.postCommandWake()
			a.clampPreviewScroll(target)
		}
		return
	default:
	}

	res := preview.Run(ctx, req)

	if runGen != gen.Load() {
		return
	}
	doneApplied := false
	a.patchPreviewState(target, func(st *ui.FilePreviewState) {
		if !st.Open || st.Path != path {
			return
		}
		st.Phase = ui.FilePreviewPhaseDone
		if res.ErrorMsg != "" {
			st.ErrorMsg = res.ErrorMsg
			st.ExitCode = res.ExitCode
			st.CombinedText = ""
			st.SetHighlightedCells(nil)
			st.IsDiff = res.IsDiff
			st.IsMarkdown = res.IsMarkdown
			st.DiffHunkLines = nil
			st.GitStatusText = ""
			st.GitStatusThemeKey = ""
			if st.Search.Active {
				st.RecomputeSearch()
			}
			doneApplied = true
			return
		}
		st.Source = res.Source
		st.CombinedText = res.CombinedText
		st.SetHighlightedCells(res.HighlightedCells)
		if res.Source == ui.PreviewSourceInternalHighlighted {
			st.ChromaStyle = req.Preview.Style
		} else {
			st.ChromaStyle = ""
		}
		st.GutterWidth = res.GutterWidth
		st.ExitCode = res.ExitCode
		st.ErrorMsg = ""
		st.IsDiff = res.IsDiff
		st.IsMarkdown = res.IsMarkdown
		st.DiffHunkLines = res.DiffHunkLines
		st.GitStatusText = res.StatusText
		st.GitStatusThemeKey = res.StatusThemeKey
		if st.Search.Active {
			st.RecomputeSearch()
		}
		doneApplied = true
	})
	if doneApplied && runGen == gen.Load() {
		a.postCommandWake()
		a.clampPreviewScroll(target)
	}
}

func (a *App) clampPreviewScroll(target previewTarget) {
	switch target {
	case previewTargetFullscreen:
		a.clampFullscreenFilePreviewScroll()
	case previewTargetCarousel:
		a.clampCarouselFilePreviewScroll()
	default:
		a.clampFilePreviewScroll()
	}
}

func (a *App) previewTextWidth(target previewTarget) (int, bool) {
	switch target {
	case previewTargetFullscreen:
		return a.fullscreenPreviewTextWidth()
	default:
		tw, _, ok := a.inactivePanelPreviewLayoutMetrics(a.filePreviewOpen())
		return tw, ok
	}
}

func (a *App) hunkScrollTo(target previewTarget, scroll int) {
	switch target {
	case previewTargetFullscreen:
		a.fullscreenPreviewScrollTo(scroll)
	default:
		a.previewScrollTo(scroll)
	}
}

// hunkNavigate scrolls the preview for target to the next (dir>0) or previous (dir<0)
// contiguous +/- change chunk (DiffHunkLines).
func (a *App) hunkNavigate(target previewTarget, dir int) {
	a.commandsMu.RLock()
	var st ui.FilePreviewState
	switch target {
	case previewTargetFullscreen:
		st = a.model.FullscreenFilePreview
	default:
		st = a.model.FilePreview
	}
	a.commandsMu.RUnlock()

	if !st.IsDiff || st.Phase != ui.FilePreviewPhaseDone {
		return
	}
	tw, ok := a.previewTextWidth(target)
	if !ok || tw < 1 {
		return
	}

	currentScroll := st.Scroll
	var targetOffset int
	found := false

	if dir > 0 {
		for _, hunkLine := range st.DiffHunkLines {
			offset := st.SourceLineToScrollOffset(hunkLine, tw, tcell.StyleDefault)
			if offset > currentScroll {
				targetOffset = offset
				found = true
				break
			}
		}
	} else {
		for _, hunkLine := range st.DiffHunkLines {
			offset := st.SourceLineToScrollOffset(hunkLine, tw, tcell.StyleDefault)
			if offset < currentScroll {
				targetOffset = offset
				found = true
			}
		}
	}

	if !found {
		if dir > 0 {
			a.setTransientMessage("No more changes", ui.MessageUrgencyInfo)
		} else {
			a.setTransientMessage("No previous changes", ui.MessageUrgencyInfo)
		}
		return
	}
	a.hunkScrollTo(target, targetOffset)
}
