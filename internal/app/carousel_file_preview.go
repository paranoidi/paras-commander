package app

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

func (a *App) closeCarouselFilePreview() {
	a.commandsMu.Lock()
	a.model.CarouselFilePreview = ui.FilePreviewState{}
	a.commandsMu.Unlock()
	a.clearFilePreviewHold(previewTargetCarousel)
	a.carouselFilePreviewRunGen.Add(1)
	a.carouselFilePreviewLastFingerprint = ""
}

func (a *App) patchCarouselFilePreview(fn func(*ui.FilePreviewState)) {
	a.patchPreviewState(previewTargetCarousel, fn)
}

func (a *App) patchCarouselFilePreviewMessage(titleBase, msg string) {
	a.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
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
	a.clampCarouselFilePreviewScroll()
}

func (a *App) carouselFilePreviewScrollMetrics() (textW, contentH, lineCount int) {
	tw, ch, layOK := a.carouselChildPreviewLayoutMetrics()
	if !layOK {
		return tw, ch, 0
	}
	textW, contentH = tw, ch
	a.commandsMu.RLock()
	ph := a.model.CarouselFilePreview.Phase
	em := a.model.CarouselFilePreview.ErrorMsg
	a.commandsMu.RUnlock()
	switch ph {
	case ui.FilePreviewPhasePending, ui.FilePreviewPhaseRunning:
		lineCount = 1
	case ui.FilePreviewPhaseDone:
		if strings.TrimSpace(em) != "" {
			lineCount = 1
			break
		}
		lineCount = a.carouselFilePreviewLineCount(textW)
		if lineCount < 1 {
			lineCount = 1
		}
	default:
		lineCount = 1
	}
	return textW, contentH, lineCount
}

func (a *App) clampCarouselFilePreviewScroll() {
	_, ch, lc := a.carouselFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (a *App) carouselPreviewScrollBy(delta int) {
	_, ch, lc := a.carouselFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	a.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll += delta
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (a *App) carouselFilePreviewScrollable() bool {
	if !a.carouselFilePreviewContext() {
		return false
	}
	a.commandsMu.RLock()
	defer a.commandsMu.RUnlock()
	st := a.model.CarouselFilePreview
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

func (a *App) activePanelFileColumnRect() (geom.Rect, bool) {
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		return geom.Rect{}, false
	}
	col := lay.Left
	if a.model.ActivePanel == ui.RightPanel {
		col = lay.Right
	}
	stripN := ui.SelectionsStripLayoutItemCount(
		a.activePanel(),
		a.model.ActivePanel,
		a.model.ActivePanel,
		a.model.ThemeDialog.Open,
	)
	fileCol, _ := ui.SplitPanelColumn(ui.Rect(col), stripN, a.model.SelectionsPanelMaxRows, ui.MinFileListContentRows)
	if fileCol.Width <= 0 {
		return geom.Rect{}, false
	}
	return geom.Rect(fileCol), true
}

func (a *App) carouselFilePreviewEligible() bool {
	rect, ok := a.activePanelFileColumnRect()
	if !ok {
		return false
	}
	return panelcarousel.FilePreviewEligible(rect, a.model.HideInactivePanel, a.model.CarouselLayout)
}

func (a *App) carouselChildPreviewLayoutMetrics() (textW, contentH int, ok bool) {
	rect, ok := a.activePanelFileColumnRect()
	if !ok {
		return 1, 0, false
	}
	childW := panelcarousel.ChildColumnWidth(rect, a.model.CarouselLayout)
	tw := childW - 2
	if tw < 1 {
		tw = 1
	}
	listH := geom.PanelListRows(rect)
	if listH < 1 {
		return tw, 0, false
	}
	return tw, listH, true
}

func (a *App) carouselFilePreviewWantPath() (path string, ok bool) {
	p := a.activePanel()
	entry, okEntry := p.CurrentEntry()
	if !okEntry || entry.Type == localfs.EntryDirectory {
		return "", false
	}
	path = filepath.Clean(entry.Path)
	if path == "" || path == "." {
		return "", false
	}
	return path, true
}

func (a *App) carouselFilePreviewFingerprint() string {
	path, ok := a.carouselFilePreviewWantPath()
	if !ok {
		return "none"
	}
	return "f:" + path
}

func (a *App) carouselFilePreviewContext() bool {
	if a.model.ViewMode != ui.ViewBrowser {
		return false
	}
	p := a.activePanel()
	if !p.CarouselMode {
		return false
	}
	if a.model.QuickViewDisplayActive() {
		return false
	}
	if !a.carouselFilePreviewEligible() {
		return false
	}
	rect, ok := a.activePanelFileColumnRect()
	if !ok || !panelcarousel.LayoutFits(rect, a.model.CarouselLayout, true) {
		return false
	}
	if !panelcarousel.ShowChildPreviewColumn(*p, false, true) {
		return false
	}
	if panelcarousel.ChildPreviewKindFor(*p, false, true) != panelcarousel.ChildPreviewFile {
		return false
	}
	if a.model.Menu.Open || a.model.ModalDialogOpen() {
		return false
	}
	if a.model.ActiveSubFocus != ui.SubFocusFileList {
		return false
	}
	if a.model.ActivePanel != ui.LeftPanel && a.model.ActivePanel != ui.RightPanel {
		return false
	}
	if _, ok := a.carouselFilePreviewWantPath(); !ok {
		return false
	}
	return true
}

func (a *App) applyCarouselFilePreviewNow() {
	if !a.carouselFilePreviewContext() {
		a.closeCarouselFilePreview()
		return
	}
	path, ok := a.carouselFilePreviewWantPath()
	if !ok {
		a.closeCarouselFilePreview()
		return
	}
	workDir := a.activePanel().PathString()
	if err := localfs.CheckFilePreviewable(path); err != nil {
		switch {
		case errors.Is(err, localfs.ErrFilePreviewBinary):
			a.patchCarouselFilePreviewMessage(filepath.Base(path), "Not a text file")
		case errors.Is(err, localfs.ErrFilePreviewIsDir):
			a.patchCarouselFilePreviewMessage("", "Not a file")
		default:
			a.patchCarouselFilePreviewMessage(filepath.Base(path), err.Error())
		}
		return
	}
	tw, _, layOK := a.carouselChildPreviewLayoutMetrics()
	if !layOK {
		tw = 1
	}
	titleBase := filepath.Base(path)
	a.captureFilePreviewHold(previewTargetCarousel)
	a.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
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
	})
	a.postCommandWake()
	gen := a.carouselFilePreviewRunGen.Add(1)
	go a.runPreview(a.commandsCtx, a.previewRequest(path, tw, workDir, a.activePanelChromeBlocked()), previewTargetCarousel, gen)
}

func (a *App) reconcileCarouselFilePreview() {
	if a.model.Menu.Open || a.model.ModalDialogOpen() {
		return
	}
	p := a.activePanel()
	if !p.CarouselMode {
		// Active panel is not in carousel mode — do not touch the
		// global carousel file preview; the inactive panel (still with
		// CarouselMode=true) may be using it for its child column.
		return
	}
	eligible := a.carouselFilePreviewEligible()
	kind := panelcarousel.ChildPreviewKindFor(*p, a.model.QuickViewDisplayActive(), eligible)
	if kind != panelcarousel.ChildPreviewFile {
		if a.inQuickFilterUI() {
			// Preserve existing preview while user navigates with a filter active.
			return
		}
		a.commandsMu.RLock()
		open := a.model.CarouselFilePreview.Open
		a.commandsMu.RUnlock()
		if open || a.carouselFilePreviewLastFingerprint != "" {
			a.closeCarouselFilePreview()
		}
		return
	}
	if a.model.ActiveSubFocus != ui.SubFocusFileList {
		return
	}
	sig := a.carouselFilePreviewFingerprint()
	if sig == a.carouselFilePreviewLastFingerprint {
		return
	}
	a.commandsMu.RLock()
	previewOpen := a.model.CarouselFilePreview.Open
	a.commandsMu.RUnlock()
	if !previewOpen {
		// Moving onto a file from a directory (or with nothing previewed yet) opens
		// the preview from closed. Debouncing here would blank the child column for
		// the debounce interval before the title paints, causing a visible flicker.
		// Apply immediately so the title renders in the same frame; the body still
		// loads asynchronously. File→file and directory navigation keep the existing
		// content visible meanwhile, so they debounce as before.
		a.clearCarouselPreviewNavCoalesce()
		a.applyCarouselFilePreviewAfterFlush()
		return
	}
	if a.carouselPreviewNavSkipSnapshot.Load() {
		return
	}
	if a.config.UI.KeyRepeatDebounceMS <= 0 {
		a.applyCarouselFilePreviewAfterFlush()
		return
	}
	a.armCarouselPreviewNavCoalesceAfterListNav()
}

func (a *App) applyCarouselFilePreviewAfterFlush() {
	if !a.carouselFilePreviewContext() {
		a.closeCarouselFilePreview()
		return
	}
	a.applyCarouselFilePreviewNow()
	a.carouselFilePreviewLastFingerprint = a.carouselFilePreviewFingerprint()
}
