package preview

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitignore"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

func (h *Handler) patchPreviewState(target previewTarget, fn func(*ui.FilePreviewState)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	switch target {
	case previewTargetFullscreen:
		fn(&h.model.FullscreenFilePreview)
	case previewTargetCarousel:
		fn(&h.model.CarouselFilePreview)
	default:
		fn(&h.model.FilePreview)
	}
}

func (h *Handler) previewRunGenFor(target previewTarget) *atomic.Uint64 {
	switch target {
	case previewTargetCarousel:
		return &h.carouselFilePreviewRunGen
	default:
		return &h.filePreviewRunGen
	}
}

// CloseFilePreview closes the inactive-column (quick view) preview.
func (h *Handler) CloseFilePreview() {
	h.mu.Lock()
	h.model.FilePreview = ui.FilePreviewState{}
	h.mu.Unlock()
	h.clearFilePreviewHold(previewTargetInactive)
	if h.model.ActiveSubFocus == ui.SubFocusInactivePreview {
		h.model.ActiveSubFocus = ui.SubFocusFileList
	}
}

// FilePreviewOpen reports whether the inactive-column preview is open.
func (h *Handler) FilePreviewOpen() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.model.FilePreview.Open
}

// inactivePanelPreviewLayoutMetrics returns inner text width and preview body row count for the
// inactive column. filePreviewOpenForLayout should match whether preview is treated as open for
// panel zoom (true = even split when zoom would otherwise apply). ok is false when layout.TooSmall.
func (h *Handler) inactivePanelPreviewLayoutMetrics(filePreviewOpenForLayout bool) (textW, contentH int, ok bool) {
	w, ht := h.screen.Size()
	lay := h.host.LayoutForTerminalSizePreview(w, ht, filePreviewOpenForLayout)
	if lay.TooSmall {
		return 1, 0, false
	}
	inactiveID := h.host.InactivePanelID()
	col := lay.Primary
	p := &h.model.Primary
	if inactiveID == ui.SecondaryPanel {
		col = lay.Secondary
		p = &h.model.Secondary
	}
	stripN := ui.SelectionsStripLayoutItemCount(p, inactiveID, h.model.ActivePanel, h.model.ThemeDialog.Open)
	fileCol, _ := ui.SplitPanelForSelections(col, ui.SelectionsStripSplitParams{
		StripItemCount:     stripN,
		MaxRows:            h.model.SelectionsPanelMaxRows,
		ActivePercent:      h.model.SelectionsPanelActivePercent,
		StripFocused:       false, // inactive column never focuses the strip
		Orientation:        h.model.SplitOrientation,
		MinFileContentRows: ui.MinFileListContentRows,
	})
	tw := fileCol.Width - 4
	if tw < 1 {
		tw = 1
	}
	return tw, ui.JobsPanelContentRows(fileCol), true
}

func (h *Handler) filePreviewScrollMetrics() (textW, contentH, lineCount int) {
	tw, ch, layOK := h.inactivePanelPreviewLayoutMetrics(h.FilePreviewOpen())
	if !layOK {
		return tw, ch, 0
	}
	textW, contentH = tw, ch
	h.mu.RLock()
	ph := h.model.FilePreview.Phase
	em := h.model.FilePreview.ErrorMsg
	h.mu.RUnlock()
	switch ph {
	case ui.FilePreviewPhasePending, ui.FilePreviewPhaseRunning:
		lineCount = 1
	case ui.FilePreviewPhaseDone:
		if strings.TrimSpace(em) != "" {
			lineCount = 1
			break
		}
		lineCount = h.filePreviewLineCount(textW)
		if lineCount < 1 {
			lineCount = 1
		}
	default:
		lineCount = 1
	}
	return textW, contentH, lineCount
}

func (h *Handler) clampFilePreviewScroll() {
	_, ch, lc := h.filePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchFilePreview(func(st *ui.FilePreviewState) {
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (h *Handler) previewScrollBy(delta int) {
	_, ch, lc := h.filePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll += delta
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (h *Handler) previewScrollTo(scroll int) {
	_, ch, lc := h.filePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll = scroll
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

// CycleSubFocusForTabWithPreview advances Tab focus among file list, selections strip (if any),
// and preview pane. Tab from preview returns to the active panel's file list (see TryDispatchFilePreviewFocus).
func (h *Handler) CycleSubFocusForTabWithPreview() {
	switch h.model.ActiveSubFocus {
	case ui.SubFocusFileList:
		if h.host.ActivePanel().SelectionsStripCount() > 0 {
			h.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
			h.host.ActivePanel().EnsureSelectionsStripCursorVisible(h.host.SelectionsStripViewportRows(h.model.ActivePanel))
			return
		}
		h.model.ActiveSubFocus = ui.SubFocusInactivePreview
	case ui.SubFocusSelectionsStrip:
		h.model.ActiveSubFocus = ui.SubFocusInactivePreview
	case ui.SubFocusInactivePreview:
		// Defensive: Tab from preview is handled in TryDispatchFilePreviewFocus before dispatch reaches here.
		h.model.ActiveSubFocus = ui.SubFocusFileList
	}
}

// TryDispatchFilePreviewFocus handles keys while keyboard focus is on the inactive preview pane.
func (h *Handler) TryDispatchFilePreviewFocus(actionID string) bool {
	if h.model.ViewMode != ui.ViewBrowser || h.model.ActiveSubFocus != ui.SubFocusInactivePreview {
		return false
	}
	if !h.FilePreviewOpen() {
		h.model.ActiveSubFocus = ui.SubFocusFileList
		return false
	}
	_, ch, _ := h.filePreviewScrollMetrics()
	step := ch
	if step < 1 {
		step = 1
	}
	switch actionID {
	case keymap.ActionNavUp:
		h.previewScrollBy(-1)
		return true
	case keymap.ActionNavDown:
		h.previewScrollBy(1)
		return true
	case keymap.ActionNavPageUp:
		h.previewScrollBy(-step)
		return true
	case keymap.ActionNavPageDown:
		h.previewScrollBy(step)
		return true
	case keymap.ActionNavTop:
		h.previewScrollTo(0)
		return true
	case keymap.ActionNavBottom:
		_, ch2, lc := h.filePreviewScrollMetrics()
		h.previewScrollTo(max(0, lc-ch2))
		return true
	case keymap.ActionPanelSwitch:
		if h.model.QuickViewEnabled {
			h.host.SwitchPanel()
		} else {
			h.model.ActiveSubFocus = ui.SubFocusFileList
		}
		return true
	case keymap.ActionNavOpen:
		return true
	case keymap.ActionPanelFocusSelections:
		if h.host.ActivePanel().SelectionsStripCount() > 0 {
			h.model.ActiveSubFocus = ui.SubFocusSelectionsStrip
			h.host.ActivePanel().EnsureSelectionsStripCursorVisible(h.host.SelectionsStripViewportRows(h.model.ActivePanel))
		} else {
			h.model.ActiveSubFocus = ui.SubFocusFileList
		}
		return true
	default:
		return false
	}
}

// quickViewFilePreviewScrollable reports whether quick view is painting scrollable file preview text.
func (h *Handler) quickViewFilePreviewScrollable() bool {
	if !h.model.QuickViewDisplayActive() || h.model.QuickViewDirOverlayActive {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	st := h.model.FilePreview
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

// TryDispatchQuickViewPreviewScroll handles Ctrl+J/Ctrl+K preview page keys.
// Scrolls carousel child preview or inactive quick-view preview when available;
// otherwise pages the active file list while quick view is latched.
func (h *Handler) TryDispatchQuickViewPreviewScroll(actionID string) bool {
	var pageDir int
	switch actionID {
	case keymap.ActionFileQuickViewPreviewPageUp:
		pageDir = -1
	case keymap.ActionFileQuickViewPreviewPageDown:
		pageDir = 1
	default:
		return false
	}
	if h.model.ViewMode != ui.ViewBrowser {
		return false
	}
	if h.carouselFilePreviewScrollable() {
		_, ch, _ := h.carouselFilePreviewScrollMetrics()
		step := ch
		if step < 1 {
			step = 1
		}
		h.carouselPreviewScrollBy(pageDir * step)
		return true
	}
	if !h.model.QuickViewEnabled {
		return false
	}
	if h.quickViewFilePreviewScrollable() {
		_, ch, _ := h.filePreviewScrollMetrics()
		step := ch
		if step < 1 {
			step = 1
		}
		h.previewScrollBy(pageDir * step)
		return true
	}
	viewportRows := h.host.ActiveViewportRows()
	h.EnsureCarouselChildCacheBeforeListNav()
	h.BeginCarouselPreviewNavCoalesce()
	h.host.ActivePanel().Page(pageDir, viewportRows)
	h.host.ArmPanelSyncFollowNavCoalesceAfterListNav()
	h.ArmQuickViewNavCoalesceAfterListNav()
	h.ArmCarouselPreviewNavCoalesceAfterListNav()
	h.host.ArmCursorNameHintNavCoalesceAfterListNav()
	return true
}

// TryDispatchFileView handles file-preview/quick-view/diff-hunk-navigation actions.
func (h *Handler) TryDispatchFileView(actionID string) bool {
	switch actionID {
	case keymap.ActionFileView:
		h.OpenFilePreviewFullscreen()
	case keymap.ActionFileViewThemePicker:
		h.toggleFilePreviewThemePicker()
	case keymap.ActionFileViewToggleRaw:
		h.toggleFilePreviewRawMarkdown()
	case keymap.ActionFileViewDiffNextHunk:
		h.hunkNavigate(previewTargetInactive, 1)
	case keymap.ActionFileViewDiffPrevHunk:
		h.hunkNavigate(previewTargetInactive, -1)
	case keymap.ActionFileQuickView:
		h.HandleQuickViewToggle()
	case keymap.ActionFileEdit:
		if h.model.ViewMode == ui.ViewFilePreview && h.model.FullscreenFilePreview.Open {
			h.host.EditFullscreenPreviewFile()
		} else {
			h.host.EditActiveFile()
		}
	default:
		return false
	}
	return true
}

func (h *Handler) patchFilePreview(fn func(*ui.FilePreviewState)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	fn(&h.model.FilePreview)
}

// HandleQuickViewToggle toggles inactive-column quick view (Shift+F3 / Left or Right menu).
func (h *Handler) HandleQuickViewToggle() {
	if h.model.ViewMode != ui.ViewBrowser {
		return
	}
	if h.model.Menu.Open || h.model.ModalDialogOpen() {
		return
	}
	if h.host.InQuickFilterUI() {
		return
	}
	active := h.model.ActivePanel
	if h.model.QuickViewEnabled && h.model.QuickViewPanel == active {
		h.model.QuickViewEnabled = false
		h.model.QuickViewPanel = -1
		h.ResetQuickViewFingerprint()
		h.CloseFilePreview()
		h.ClearQuickViewDirOverlay()
		h.host.SetTransientMessage("Quick view off", ui.MessageUrgencyInfo)
		return
	}
	h.model.QuickViewEnabled = true
	h.model.QuickViewPanel = active
	hadSync := h.model.SyncFollowEnabled
	if hadSync {
		h.model.SyncFollowEnabled = false
		h.host.ClearPanelSyncFollowNavCoalesce()
	}
	hadHidden := h.model.HideInactivePanel
	if hadHidden {
		h.model.HideInactivePanel = false
	}
	switch {
	case hadSync && hadHidden:
		h.host.SetTransientMessage("Quick view on — sync disabled, panel shown", ui.MessageUrgencyWarn)
	case hadSync:
		h.host.SetTransientMessage("Quick view on — sync disabled", ui.MessageUrgencyWarn)
	case hadHidden:
		h.host.SetTransientMessage("Quick view on — panel shown", ui.MessageUrgencyInfo)
	default:
		h.host.SetTransientMessage("Quick view on", ui.MessageUrgencyInfo)
	}
	h.ClearCarouselPreviewNavCoalesce()
	h.CloseCarouselFilePreview()
	h.ApplyQuickViewPreviewImmediately()
}

// HandlePanelDirChanged disables quick view when the non-driver (inactive) panel navigates to
// a new directory, since quick view would otherwise overlay the freshly opened listing with a
// stale preview. Invoked from App.reconcileAfterEvent for both panels every Run-loop iteration.
// Idempotent: no-ops when the flag is off, quick view isn't latched, or the path is unchanged.
func (h *Handler) HandlePanelDirChanged(panelID int) {
	if !h.host.Config().Preview.QuickViewDisableOnInactiveNav {
		return
	}
	cur := filepath.Clean(h.host.PanelByID(panelID).PathString())
	prev := h.quickViewDirNavPath[panelID]
	h.quickViewDirNavPath[panelID] = cur
	if prev == "" || prev == cur {
		return // no previously observed path (first sight) or unchanged — nothing to react to.
	}
	if !h.model.QuickViewEnabled || h.model.QuickViewPanel == panelID {
		return
	}
	h.model.QuickViewEnabled = false
	h.model.QuickViewPanel = -1
	h.ResetQuickViewFingerprint()
	h.CloseFilePreview()
	h.ClearQuickViewDirOverlay()
	h.host.SetTransientMessage("Quick view off — panel navigated", ui.MessageUrgencyInfo)
}

// PauseQuickViewDisplay hides inactive-column preview while quick view stays latched on the driver panel.
func (h *Handler) PauseQuickViewDisplay() {
	h.ResetQuickViewFingerprint()
	h.CloseFilePreview()
	h.ClearQuickViewDirOverlay()
	if h.model.ActiveSubFocus == ui.SubFocusInactivePreview {
		h.model.ActiveSubFocus = ui.SubFocusFileList
	}
}

// ResumeQuickViewDisplay restores inactive-column preview after returning to the quick-view driver panel.
func (h *Handler) ResumeQuickViewDisplay() {
	h.ClearCarouselPreviewNavCoalesce()
	h.CloseCarouselFilePreview()
	h.ApplyQuickViewPreviewImmediately()
}

// ResetQuickViewFingerprint clears any pending quick-view debounce and forgets the last applied
// fingerprint, so the next reconcile always reapplies the preview from scratch.
func (h *Handler) ResetQuickViewFingerprint() {
	h.clearQuickViewDebounce()
	h.quickViewLastFingerprint = ""
}

// DisableQuickViewDisplay tears down the inactive-column preview and its directory overlay
// (used when quick view is displaced by another exclusive panel feature, e.g. hiding the
// inactive panel or enabling latched panel sync).
func (h *Handler) DisableQuickViewDisplay() {
	h.clearQuickViewDebounce()
	h.CloseFilePreview()
	h.ClearQuickViewDirOverlay()
	h.quickViewLastFingerprint = ""
}

func (h *Handler) patchColumnPreviewMessage(titleBase, msg string) {
	h.patchFilePreview(func(st *ui.FilePreviewState) {
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
		st.ImagePayload = ""
		st.ImagePxW = 0
		st.ImagePxH = 0
		st.ImageProtocol = 0
		st.ImageUnicodePlaceholder = false
		st.ImageInTmux = false
		st.ImageCapabilityUncertain = false
	})
	h.postRenderWake()
	h.clampFilePreviewScroll()
}

// quickViewFingerprint identifies the current quick-view highlight for debouncing.
func (h *Handler) quickViewFingerprint() string {
	path, _, mode := h.quickViewWantFile()
	switch mode {
	case quickViewWantNone:
		return "none"
	case quickViewWantFile:
		return "f:" + path
	case quickViewWantDir:
		if path, ok := h.host.SyncFollowTargetPath(h.host.ActivePanel()); ok {
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
func (h *Handler) quickViewWantFile() (path string, workDir string, mode quickViewWantMode) {
	p := h.host.ActivePanel()
	workDir = p.PathString()
	if h.model.ActiveSubFocus == ui.SubFocusSelectionsStrip && p.SelectionsStripCount() > 0 {
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

// ClearQuickViewDirOverlay clears the inactive-column directory overlay state.
func (h *Handler) ClearQuickViewDirOverlay() {
	h.model.QuickViewDirOverlay = panel.State{}
	h.model.QuickViewDirOverlayActive = false
	h.model.QuickViewDirOverlayPanelID = -1
}

// populateQuickViewDirOverlay fills the inactive-column directory overlay. When driver or
// follower already lists the target directory, the live cursor is mirrored. Otherwise the
// listing is built with the same snapshot path as carousel child preview (history recall).
func (h *Handler) populateQuickViewDirOverlay(ov *panel.State, driver, follower *panel.State, dir string, panelID int) error {
	canonical := panel.CleanPathString(dir)
	if canonical == "" {
		return errors.New("empty directory path")
	}
	h.initQuickViewDirOverlayFromFollower(ov, driver, follower, panelID)
	if follower != nil && follower.ListingAtPath(canonical) {
		ov.CloneListingFrom(follower)
		return nil
	}
	if driver != nil && driver.ListingAtPath(canonical) {
		ov.CloneListingFrom(driver)
		return nil
	}
	vr := h.host.PanelViewportRows(panelID)
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
	h.primeQuickViewGitColumn(ov, canonical)
	ov.EnsureCursorInViewport(vr)
	return nil
}

// primeQuickViewGitColumn reserves the git status column on the overlay's first synchronous
// frame (a cheap, cache-backed work-tree check — no git subprocess) so the listing doesn't flash
// without the column while the real async listing load (dispatched by ov.Load above) is still in
// flight. Cell content is filled in immediately from a synchronous cache peek when available,
// otherwise it stays blank until the async git-status fetch that ApplyListing/prepareGitColumn
// dispatches once the async listing lands.
func (h *Handler) primeQuickViewGitColumn(ov *panel.State, dir string) {
	workRoot := gitignore.ValidWorkTreeRoot(dir)
	ov.GitColumnActive = workRoot != ""
	if !ov.GitColumnActive {
		return
	}
	paths := make([]gitstatus.ListingPaths, len(ov.Entries))
	for i, e := range ov.Entries {
		paths[i] = gitstatus.ListingPaths{AbsPath: filepath.Clean(e.Path), IsDir: e.Type == localfs.EntryDirectory}
	}
	if byPath, ok := h.host.PeekGitStatus(workRoot, dir, paths); ok {
		ov.GitByPath = byPath
	}
}

// initQuickViewDirOverlayFromFollower prepares QuickViewDirOverlay for a directory preview load.
// The real inactive panel path, cursor, and selection are not modified. Async listing/git-status
// requests the overlay dispatches are scheduled and applied through the same per-panel-ID
// machinery as the two real panels, via the synthetic ui.QuickViewOverlayPanel ID — followerID
// (the real inactive panel this overlay stands in for) is only used for viewport-row sizing,
// which must keep following the real follower panel's layout slot, not the overlay's.
func (h *Handler) initQuickViewDirOverlayFromFollower(ov *panel.State, driver, follower *panel.State, followerID int) {
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
		ScheduleAsyncLoad:          h.host.AsyncLoadScheduler(ui.QuickViewOverlayPanel),
		IdleDiskTotalsSort:         follower.IdleDiskTotalsSort,
		DiskUsageIdleSortEligible:  follower.DiskUsageIdleSortEligible,
		DiskUsageIdleSortActivated: follower.DiskUsageIdleSortActivated,
		HistoryCursorByPath:        panel.MergeHistoryCursorByPath(follower.HistoryCursorByPath, driver.HistoryCursorByPath),
		ScheduleGitStatus:          h.host.GitStatusScheduler(ui.QuickViewOverlayPanel),
	}
	ov.FileListViewportRows = func() int { return h.host.PanelViewportRows(followerID) }
}

// quickViewFollowDirectory loads the highlighted directory into the inactive-column overlay
// (same target-path rules and Load semantics as latched panel sync; real panel unchanged).
func (h *Handler) quickViewFollowDirectory() {
	driver := h.host.ActivePanel()
	targetPath, ok := h.host.SyncFollowTargetPath(driver)
	if !ok {
		h.ClearQuickViewDirOverlay()
		h.patchColumnPreviewMessage("", "Quick view: select a folder")
		return
	}
	h.CloseFilePreview()
	followerID := h.host.InactivePanelID()
	follower := h.host.PanelByID(followerID)
	targetPath = panel.CleanPathString(targetPath)
	if targetPath == "" {
		return
	}
	if h.host.PathVolumeContendsWithActiveJob(targetPath) {
		return
	}
	ov := &h.model.QuickViewDirOverlay
	h.model.QuickViewDirOverlayPanelID = followerID
	if err := h.populateQuickViewDirOverlay(ov, driver, follower, targetPath, followerID); err != nil {
		return
	}
	h.model.QuickViewDirOverlayActive = true
}

// ApplyQuickViewPreviewImmediately applies the current quick-view target without debouncing.
func (h *Handler) ApplyQuickViewPreviewImmediately() {
	if !h.model.QuickViewDisplayActive() || h.model.ViewMode != ui.ViewBrowser {
		return
	}
	if h.model.Menu.Open || h.model.ModalDialogOpen() || h.host.InQuickFilterUI() {
		return
	}
	h.applyQuickViewPreviewNow()
	h.quickViewLastFingerprint = h.quickViewFingerprint()
}

func (h *Handler) applyQuickViewPreviewNow() {
	if !h.model.QuickViewDisplayActive() || h.model.ViewMode != ui.ViewBrowser {
		return
	}
	if h.model.Menu.Open || h.model.ModalDialogOpen() {
		return
	}
	path, workDir, mode := h.quickViewWantFile()
	switch mode {
	case quickViewWantNone:
		h.ClearQuickViewDirOverlay()
		h.patchColumnPreviewMessage("", "Quick view: no file")
	case quickViewWantDir:
		h.quickViewFollowDirectory()
	case quickViewWantStatErr:
		h.ClearQuickViewDirOverlay()
		h.patchColumnPreviewMessage("", "Quick view: cannot read selection")
	case quickViewWantFile:
		h.ClearQuickViewDirOverlay()
		tw, contentH, layOK := h.inactivePanelPreviewLayoutMetrics(true)
		if !layOK {
			tw = 1
		}
		titleBase := filepath.Base(path)
		h.captureFilePreviewHold(previewTargetInactive)
		h.patchFilePreview(func(st *ui.FilePreviewState) {
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
			// Keep ImagePayload* so the previous image stays on screen until the new
			// encode finishes (stale-while-revalidate). Cleared on error / non-image result.
		})
		h.postRenderWake()
		gen := h.filePreviewRunGen.Add(1)
		req := h.previewRequest(path, tw, contentH, workDir, h.inactivePreviewChromeBlocked(), h.gitStatusForPath(path), previewTargetInactive)
		go h.dispatchQuickViewFilePreview(path, req, gen)
	}
}

// dispatchQuickViewFilePreview is dispatchFilePreviewCheck for the inactive-column quick view.
func (h *Handler) dispatchQuickViewFilePreview(path string, req previewrun.Request, gen uint64) {
	h.dispatchFilePreviewCheck(path, req, previewTargetInactive, gen,
		"Quick view: not a text file", "Quick view: not a file", h.patchColumnPreviewMessage)
}

// dispatchFilePreviewCheck runs CheckFilePreviewable (a stat/open/read that can block for a long
// time on a slow filesystem such as a network mount) off the UI goroutine for target, so
// highlighting a new file never blocks input handling — only the "Pending" placeholder state is
// set synchronously by the caller. gen guards against a superseded dispatch (the user already
// moved on to another file) clobbering fresher preview content once its slow I/O finally
// completes. notTextMsg/notFileMsg are target's wording for the two not-previewable cases.
func (h *Handler) dispatchFilePreviewCheck(path string, req previewrun.Request, target previewTarget, gen uint64, notTextMsg, notFileMsg string, patchMessage func(titleBase, msg string)) {
	err := localfs.CheckFilePreviewable(path)
	if gen != h.previewRunGenFor(target).Load() {
		return
	}
	isImage := errors.Is(err, localfs.ErrFilePreviewImage)
	isMedia := errors.Is(err, localfs.ErrFilePreviewMedia)
	if err != nil && !isImage && !isMedia {
		switch {
		case errors.Is(err, localfs.ErrFilePreviewBinary):
			patchMessage(filepath.Base(path), notTextMsg)
		case errors.Is(err, localfs.ErrFilePreviewIsDir):
			patchMessage("", notFileMsg)
		default:
			patchMessage(filepath.Base(path), err.Error())
		}
		return
	}
	h.runPreview(h.ctx, req, target, gen)
}

// refreshInactiveFilePreview re-runs the current inactive-column (quick view) preview at its
// current path, e.g. after a terminal resize changes the inactive column's text width. Scroll is
// left untouched (unlike applyQuickViewPreviewNow, which is for opening/switching files).
func (h *Handler) refreshInactiveFilePreview() {
	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()
	if !st.Open || st.Path == "" {
		return
	}
	tw, contentH, ok := h.inactivePanelPreviewLayoutMetrics(true)
	if !ok {
		return
	}
	workDir := h.host.ActivePanel().PathString()
	req := h.previewRequest(st.Path, tw, contentH, workDir, h.inactivePreviewChromeBlocked(), h.gitStatusForPath(st.Path), previewTargetInactive)
	gen := h.filePreviewRunGen.Add(1)
	h.postRenderWake()
	go h.runPreview(h.ctx, req, previewTargetInactive, gen)
}

// RefreshPreviewsAfterResize re-runs any open preview target whose current computed text width
// differs from the width its content was last requested at. Markdown word-wrap and table layout
// (internal/preview/mdformat) are baked into the emitted cells at request time, so a plain re-wrap
// at the new width (the downstream character-wrap cache) is not enough after a terminal resize.
func (h *Handler) RefreshPreviewsAfterResize() {
	h.refreshPreviewTargetAfterResize(previewTargetInactive)
	h.refreshPreviewTargetAfterResize(previewTargetFullscreen)
	h.refreshPreviewTargetAfterResize(previewTargetCarousel)
}

func (h *Handler) refreshPreviewTargetAfterResize(target previewTarget) {
	h.mu.RLock()
	var open bool
	var path string
	switch target {
	case previewTargetFullscreen:
		open = h.model.FullscreenFilePreview.Open
		path = h.model.FullscreenFilePreview.Path
	case previewTargetCarousel:
		open = h.model.CarouselFilePreview.Open
		path = h.model.CarouselFilePreview.Path
	default:
		open = h.model.FilePreview.Open
		path = h.model.FilePreview.Path
	}
	h.mu.RUnlock()
	if !open {
		return
	}

	var tw int
	var ok bool
	switch target {
	case previewTargetFullscreen:
		tw, ok = h.fullscreenPreviewTextWidth()
	case previewTargetCarousel:
		tw, _, ok = h.carouselChildPreviewLayoutMetrics()
	default:
		tw, _, ok = h.inactivePanelPreviewLayoutMetrics(true)
	}
	isImageOrMedia := localfs.IsImagePath(path) || localfs.IsMediaPath(path)
	// tmux frees every natively-stored Sixel image on any pane resize unconditionally
	// (screen_resize_cursor -> image_free_all in tmux's screen.c) — it does not retain or
	// redraw them itself afterward (verified against tmux source and by a real WezTerm+tmux
	// capture: a one-shot `chafa --format sixel` image is lost on resize and never comes
	// back). Skipping the eager re-encode+resend here means a bare-native-sixel preview simply
	// goes blank after a resize under tmux, matching what a one-shot sender like chafa does,
	// until something else reloads it — instead of a guaranteed extra decode/encode/transmit on
	// every single resize event.
	skipNativeSixelResizeRefresh := isImageOrMedia && previewrun.TmuxSupportsNativeSixel(os.Getenv)
	// Image/media pixel budget depends on height too; bypass the width-equality skip.
	if !ok || (tw == h.previewLastWidth[target] && !isImageOrMedia) || skipNativeSixelResizeRefresh {
		return
	}

	if isImageOrMedia {
		// The stale payload (and any held copy of it) was encoded for the pre-resize pixel
		// budget; drawing it at the newly computed placement would show a wrong-size image
		// until the re-encode lands. Clear both so Draw() renders blank/pending instead,
		// matching the first-open state.
		h.clearFilePreviewHold(target)
		h.patchPreviewState(target, func(st *ui.FilePreviewState) {
			st.Phase = ui.FilePreviewPhasePending
			st.ImagePayload = ""
			st.ImagePxW = 0
			st.ImagePxH = 0
			st.ImageProtocol = 0
			st.ImageUnicodePlaceholder = false
			st.ImageInTmux = false
			st.ImageCapabilityUncertain = false
		})
	}

	switch target {
	case previewTargetFullscreen:
		h.refreshFullscreenFilePreview()
	case previewTargetCarousel:
		h.refreshCarouselFilePreview()
	default:
		h.refreshInactiveFilePreview()
	}
}

func (h *Handler) clearQuickViewDebounce() {
	h.quickViewDebounce.Clear()
	h.quickViewDebounceGen.Add(1)
	h.quickViewNavSkipReconcile.Store(false)
}

// QuickViewNavSkipReconcile reports whether file-list nav coalesce is holding a pending quick
// view preview flush (render.go skips the inactive-column repaint while this is true).
func (h *Handler) QuickViewNavSkipReconcile() bool {
	return h.quickViewNavSkipReconcile.Load()
}

// quickViewNavCoalesceContext is true when file-list nav should coalesce quick view preview updates.
func (h *Handler) quickViewNavCoalesceContext() bool {
	return h.model.ViewMode == ui.ViewBrowser &&
		h.model.QuickViewDisplayActive() &&
		h.model.ActiveSubFocus == ui.SubFocusFileList &&
		!h.model.Menu.Open &&
		!h.model.ModalDialogOpen() &&
		!h.host.InQuickFilterUI()
}

// ClearQuickViewNavCoalesce stops pending file-list nav coalesce and allows reconcile to preview again.
func (h *Handler) ClearQuickViewNavCoalesce() {
	h.clearQuickViewDebounce()
}

// ClearNavCoalesces stops both pending quick-view and carousel side-preview nav coalesce debounces
// unconditionally (used on terminal resize and the global-quit-shortcut cleanup path).
func (h *Handler) ClearNavCoalesces() {
	h.clearQuickViewDebounce()
	h.clearCarouselPreviewDebounce()
}

func (h *Handler) scheduleQuickViewDebounceTimer(gen uint64) {
	delay := time.Duration(h.host.Config().UI.KeyRepeatDebounceMS) * time.Millisecond
	h.quickViewDebounce.Reset(delay, func() {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(QuickViewFlushPayload{gen: gen}))
	})
}

// ArmQuickViewNavCoalesceAfterListNav arms the quick-view preview coalesce debounce after a
// file-list cursor move, when currently in quick-view nav-coalesce context.
func (h *Handler) ArmQuickViewNavCoalesceAfterListNav() {
	if h.host.Config().UI.KeyRepeatDebounceMS <= 0 {
		return
	}
	if !h.quickViewNavCoalesceContext() {
		return
	}
	gen := h.quickViewDebounceGen.Add(1)
	h.quickViewNavSkipReconcile.Store(true)
	h.scheduleQuickViewDebounceTimer(gen)
}

func (h *Handler) armQuickViewPreviewDebounce() {
	if h.host.Config().UI.KeyRepeatDebounceMS <= 0 {
		h.applyQuickViewPreviewNow()
		h.quickViewLastFingerprint = h.quickViewFingerprint()
		return
	}
	gen := h.quickViewDebounceGen.Add(1)
	h.scheduleQuickViewDebounceTimer(gen)
}

// ApplyQuickViewPreviewFlush applies the debounced quick-view preview reload. Returns true when a
// repaint is needed.
func (h *Handler) ApplyQuickViewPreviewFlush(p QuickViewFlushPayload) bool {
	if p.gen != h.quickViewDebounceGen.Load() {
		return false
	}
	h.quickViewNavSkipReconcile.Store(false)
	if !h.model.QuickViewDisplayActive() || h.model.ViewMode != ui.ViewBrowser {
		return false
	}
	if h.model.Menu.Open || h.model.ModalDialogOpen() {
		return false
	}
	h.applyQuickViewPreviewNow()
	h.quickViewLastFingerprint = h.quickViewFingerprint()
	return true
}

// FlushQuickViewPreviewNow applies the currently pending quick-view preview debounce immediately
// (skips waiting for the timer), for callers that need synchronous flush semantics.
func (h *Handler) FlushQuickViewPreviewNow() bool {
	return h.ApplyQuickViewPreviewFlush(QuickViewFlushPayload{gen: h.quickViewDebounceGen.Load()})
}

// ReconcileQuickViewPreview reapplies the inactive-column quick-view preview when its fingerprint
// has changed since the last apply. Called once per event from reconcileAfterEvent.
func (h *Handler) ReconcileQuickViewPreview() {
	if !h.model.QuickViewDisplayActive() || h.model.ViewMode != ui.ViewBrowser {
		h.clearQuickViewDebounce()
		return
	}
	if h.quickViewNavSkipReconcile.Load() {
		return
	}
	if h.model.Menu.Open || h.model.ModalDialogOpen() {
		return
	}
	sig := h.quickViewFingerprint()
	if sig == h.quickViewLastFingerprint {
		return
	}
	h.armQuickViewPreviewDebounce()
}

func (h *Handler) previewRequest(path string, textW, contentH int, workDir string, chromeBlocked bool, gitStatus *gitstatus.Cell, target previewTarget) previewrun.Request {
	h.previewLastWidth[target] = textW
	req := previewrun.Request{
		Path:      path,
		TextWidth: textW,
		WorkDir:   workDir,
		Preview:   h.host.Config().Preview,
		BaseStyle: ui.FilePreviewBodyStyle(h.host.Styles(), chromeBlocked),
	}
	isImage := localfs.IsImagePath(path)
	isMedia := localfs.IsMediaPath(path)
	if isImage || isMedia {
		cw, ch := previewpanel.CellPixelDims(h.screen)
		req.ImageMaxPxW = textW * cw
		req.ImageMaxPxH = contentH * ch
		req.ImageCellPxH = ch
		req.ImageInTmux = os.Getenv("TMUX") != ""
		if _, ok := h.screen.Tty(); !ok {
			req.ImageProtocol = previewpanel.ImageProtocolNone
		} else if isMedia {
			req.Media = true
			req.ImageProtocol = previewrun.ResolveVideoThumbProtocol(req.Preview.Images, req.Preview, os.Getenv)
		} else {
			req.Image = true
			if !req.Preview.Images {
				req.ImageProtocol = previewpanel.ImageProtocolNone
			} else {
				req.ImageProtocol = previewrun.ResolveImageProtocol(req.Preview, os.Getenv)
			}
		}
		req.ImageUnicodePlaceholder = req.ImageProtocol == previewpanel.ImageProtocolKitty &&
			previewrun.TmuxSupportsKittyUnicodePlaceholders(os.Getenv, req.Preview)
		req.ImageCapabilityUncertain = previewrun.CapabilityUncertain(req.Preview, os.Getenv)
		req.Cache = h.mediaCache()
		return req
	}
	if gitStatus != nil {
		req.GitDiff = true
		req.GitStatus = gitStatus
	}
	return req
}

// gitStatusForPath returns the git status for path from the active panel, or nil if unavailable.
func (h *Handler) gitStatusForPath(path string) *gitstatus.Cell {
	p := h.host.ActivePanel()
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

func (h *Handler) runPreview(ctx context.Context, req previewrun.Request, target previewTarget, runGen uint64) {
	gen := h.previewRunGenFor(target)
	path := req.Path
	if runGen != gen.Load() {
		return
	}
	runningApplied := false
	h.patchPreviewState(target, func(st *ui.FilePreviewState) {
		if !st.Open || st.Path != path {
			return
		}
		st.Phase = ui.FilePreviewPhaseRunning
		runningApplied = true
	})
	if runningApplied && runGen == gen.Load() {
		h.postRenderWake()
	}

	select {
	case <-ctx.Done():
		if runGen != gen.Load() {
			return
		}
		canceledApplied := false
		h.patchPreviewState(target, func(st *ui.FilePreviewState) {
			if st.Path != path {
				return
			}
			st.Phase = ui.FilePreviewPhaseDone
			st.ErrorMsg = "Canceled"
			canceledApplied = true
		})
		if canceledApplied && runGen == gen.Load() {
			h.postRenderWake()
			h.clampPreviewScroll(target)
		}
		return
	default:
	}

	if req.Media {
		h.runMediaPreview(ctx, req, target, runGen)
		return
	}

	res := previewrun.Run(ctx, req)
	h.applyPreviewResult(req, target, runGen, res)
}

func (h *Handler) runMediaPreview(ctx context.Context, req previewrun.Request, target previewTarget, runGen uint64) {
	meta, work := previewrun.RunMediaMeta(req)
	if meta.ErrorMsg != "" {
		h.applyPreviewResult(req, target, runGen, meta)
		return
	}
	if work != nil {
		if !mediaThumbWarm(req) {
			pending := meta
			pending.CombinedText = meta.CombinedText + "\n\n" + previewrun.GeneratingThumbnailsLabel + " ..."
			// Phase=Done so MergeDrawWithHold does not replace this body with a prior hold.
			h.applyPreviewResult(req, target, runGen, pending)
		}

		select {
		case <-ctx.Done():
			h.applyPreviewResult(req, target, runGen, previewrun.Result{ErrorMsg: "Canceled"})
			return
		default:
		}
		gen := h.previewRunGenFor(target)
		metaText := meta.CombinedText
		onProgress := func(done, total int) {
			if runGen != gen.Load() {
				return
			}
			applied := false
			h.patchPreviewState(target, func(st *ui.FilePreviewState) {
				if !st.Open || st.Path != req.Path {
					return
				}
				st.CombinedText = metaText + "\n\n" + fmt.Sprintf("%s (%d/%d) ...", previewrun.GeneratingThumbnailsLabel, done, total)
				applied = true
			})
			if applied && runGen == gen.Load() {
				h.postRenderWake()
			}
		}
		res := previewrun.RunMediaThumbs(ctx, req, work, onProgress)
		h.applyPreviewResult(req, target, runGen, res)
		return
	}
	h.applyPreviewResult(req, target, runGen, meta)
}

// mediaThumbWarm reports whether the video thumb grid is already in the shared cache
// (memory or disk), so the UI can skip the "Generating thumbnails…" interim line.
func mediaThumbWarm(req previewrun.Request) bool {
	if req.Cache == nil {
		return false
	}
	fi, err := os.Stat(req.Path)
	if err != nil {
		return false
	}
	cols := req.Preview.VideoThumbCols
	rows := req.Preview.VideoThumbRows
	if cols < 1 {
		cols = 2
	}
	if rows < 1 {
		rows = 2
	}
	maxEdge := previewrun.EffectiveVideoThumbMaxEdge(req.Preview, req.ImageProtocol, req.ImageInTmux)
	return req.Cache.HasVideo(req.Path, fi.ModTime().UnixNano(), fi.Size(), maxEdge, cols, rows)
}

func (h *Handler) applyPreviewResult(req previewrun.Request, target previewTarget, runGen uint64, res previewrun.Result) {
	gen := h.previewRunGenFor(target)
	path := req.Path
	if runGen != gen.Load() {
		return
	}
	doneApplied := false
	h.patchPreviewState(target, func(st *ui.FilePreviewState) {
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
			st.ImagePayload = ""
			st.ImagePxW = 0
			st.ImagePxH = 0
			st.ImageProtocol = 0
			st.ImageUnicodePlaceholder = false
			st.ImageInTmux = false
			st.ImageCapabilityUncertain = false
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
		st.ImagePayload = res.ImagePayload
		st.ImagePxW = res.ImagePxW
		st.ImagePxH = res.ImagePxH
		st.ImageProtocol = res.ImageProtocol
		st.ImageUnicodePlaceholder = res.ImageUnicodePlaceholder
		st.ImageInTmux = res.ImageInTmux
		st.ImageCapabilityUncertain = res.ImageCapabilityUncertain
		if st.Search.Active {
			st.RecomputeSearch()
		}
		doneApplied = true
	})
	if doneApplied && runGen == gen.Load() {
		h.postRenderWake()
		h.clampPreviewScroll(target)
	}
}

func (h *Handler) clampPreviewScroll(target previewTarget) {
	switch target {
	case previewTargetFullscreen:
		h.ClampFullscreenFilePreviewScroll()
	case previewTargetCarousel:
		h.clampCarouselFilePreviewScroll()
	default:
		h.clampFilePreviewScroll()
	}
}

func (h *Handler) previewTextWidth(target previewTarget) (int, bool) {
	switch target {
	case previewTargetFullscreen:
		return h.fullscreenPreviewTextWidth()
	default:
		tw, _, ok := h.inactivePanelPreviewLayoutMetrics(h.FilePreviewOpen())
		return tw, ok
	}
}

func (h *Handler) hunkScrollTo(target previewTarget, scroll int) {
	switch target {
	case previewTargetFullscreen:
		h.fullscreenPreviewScrollTo(scroll)
	default:
		h.previewScrollTo(scroll)
	}
}

// hunkNavigate scrolls the preview for target to the next (dir>0) or previous (dir<0)
// contiguous +/- change chunk (DiffHunkLines).
func (h *Handler) hunkNavigate(target previewTarget, dir int) {
	h.mu.RLock()
	var st ui.FilePreviewState
	switch target {
	case previewTargetFullscreen:
		st = h.model.FullscreenFilePreview
	default:
		st = h.model.FilePreview
	}
	h.mu.RUnlock()

	if !st.IsDiff || st.Phase != ui.FilePreviewPhaseDone {
		return
	}
	tw, ok := h.previewTextWidth(target)
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
			h.host.SetTransientMessage("No more changes", ui.MessageUrgencyInfo)
		} else {
			h.host.SetTransientMessage("No previous changes", ui.MessageUrgencyInfo)
		}
		return
	}
	h.hunkScrollTo(target, targetOffset)
}
