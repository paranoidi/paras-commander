package preview

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

// CloseCarouselFilePreview closes the carousel child-column preview.
func (h *Handler) CloseCarouselFilePreview() {
	h.mu.Lock()
	h.model.CarouselFilePreview = ui.FilePreviewState{}
	h.mu.Unlock()
	h.clearFilePreviewHold(previewTargetCarousel)
	h.carouselFilePreviewRunGen.Add(1)
	h.carouselFilePreviewLastFingerprint = ""
}

func (h *Handler) patchCarouselFilePreview(fn func(*ui.FilePreviewState)) {
	h.patchPreviewState(previewTargetCarousel, fn)
}

func (h *Handler) patchCarouselFilePreviewMessage(titleBase, msg string) {
	h.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Open = true
		st.Phase = ui.FilePreviewPhaseDone
		st.Path = ""
		st.TitleBase = titleBase
		st.CombinedText = ""
		st.Scroll = 0
		st.ExitCode = 0
		st.ErrorMsg = msg
		st.ImagePayload = ""
		st.ImagePxW = 0
		st.ImagePxH = 0
		st.ImageProtocol = 0
	})
	h.postRenderWake()
	h.clampCarouselFilePreviewScroll()
}

func (h *Handler) carouselFilePreviewScrollMetrics() (textW, contentH, lineCount int) {
	tw, ch, layOK := h.carouselChildPreviewLayoutMetrics()
	if !layOK {
		return tw, ch, 0
	}
	textW, contentH = tw, ch
	h.mu.RLock()
	ph := h.model.CarouselFilePreview.Phase
	em := h.model.CarouselFilePreview.ErrorMsg
	h.mu.RUnlock()
	switch ph {
	case ui.FilePreviewPhasePending, ui.FilePreviewPhaseRunning:
		lineCount = 1
	case ui.FilePreviewPhaseDone:
		if strings.TrimSpace(em) != "" {
			lineCount = 1
			break
		}
		lineCount = h.carouselFilePreviewLineCount(textW)
		if lineCount < 1 {
			lineCount = 1
		}
	default:
		lineCount = 1
	}
	return textW, contentH, lineCount
}

func (h *Handler) clampCarouselFilePreviewScroll() {
	_, ch, lc := h.carouselFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (h *Handler) carouselPreviewScrollBy(delta int) {
	_, ch, lc := h.carouselFilePreviewScrollMetrics()
	maxStart := max(0, lc-ch)
	h.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
		st.Scroll += delta
		if st.Scroll < 0 {
			st.Scroll = 0
		}
		if st.Scroll > maxStart {
			st.Scroll = maxStart
		}
	})
}

func (h *Handler) carouselFilePreviewScrollable() bool {
	if !h.carouselFilePreviewContext() {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	st := h.model.CarouselFilePreview
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

func (h *Handler) activePanelFileColumnRect() (geom.Rect, bool) {
	w, ht := h.screen.Size()
	lay := h.host.LayoutForTerminalSize(w, ht)
	if lay.TooSmall {
		return geom.Rect{}, false
	}
	col := lay.Primary
	if h.model.ActivePanel == ui.SecondaryPanel {
		col = lay.Secondary
	}
	stripN := ui.SelectionsStripLayoutItemCount(
		h.host.ActivePanel(),
		h.model.ActivePanel,
		h.model.ActivePanel,
		h.model.ThemeDialog.Open,
	)
	fileCol, _ := ui.SplitPanelColumn(ui.Rect(col), stripN, h.model.SelectionsPanelMaxRows, ui.MinFileListContentRows)
	if fileCol.Width <= 0 {
		return geom.Rect{}, false
	}
	return geom.Rect(fileCol), true
}

func (h *Handler) carouselFilePreviewEligible() bool {
	rect, ok := h.activePanelFileColumnRect()
	if !ok {
		return false
	}
	return panelcarousel.FilePreviewEligible(rect, h.model.HideInactivePanel || h.host.CarouselAutohideInactivePanel(), h.model.CarouselLayout)
}

// carouselChildPreviewLayoutMetrics returns the embedded file preview's text width and content
// height inside the carousel child column.
//
// ponytail: measures the parent/center columns' actual fit-to-content widths (mirroring
// drawPanelCarousel's MeasureFitColumnWidths + ChildPreviewPaintRect call) rather than
// panelcarousel.ChildColumnWidth's unmeasured worst-case cap — otherwise a fit-mode split
// (e.g. the default "<33%") that measures narrower than its cap leaves the child preview
// pre-wrapped to a too-small width while the painted column is actually wider.
func (h *Handler) carouselChildPreviewLayoutMetrics() (textW, contentH int, ok bool) {
	rect, ok := h.activePanelFileColumnRect()
	if !ok {
		return 1, 0, false
	}
	listH := geom.PanelListRows(rect)
	if listH < 1 {
		return 1, 0, false
	}
	state := *h.host.ActivePanel()
	parent, _, _, _ := panelcarousel.BuildColumns(state, listH, false, true)
	measuredFitWidth := panelcarousel.MeasureFitColumnWidths(h.model.CarouselLayout, parent, state, h.model.ShowFileIcons, true, h.model.PanelScrollbar, listH)
	childRect, ok := panelcarousel.ChildPreviewPaintRect(rect, true, h.model.CarouselLayout, measuredFitWidth)
	if !ok {
		return 1, listH, false
	}
	tw := childRect.Width - 2
	if tw < 1 {
		tw = 1
	}
	return tw, listH, true
}

func (h *Handler) carouselFilePreviewWantPath() (path string, ok bool) {
	p := h.host.ActivePanel()
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

func (h *Handler) carouselFilePreviewFingerprint() string {
	path, ok := h.carouselFilePreviewWantPath()
	if !ok {
		return "none"
	}
	return "f:" + path
}

func (h *Handler) carouselFilePreviewContext() bool {
	if h.model.ViewMode != ui.ViewBrowser {
		return false
	}
	p := h.host.ActivePanel()
	if !p.CarouselMode {
		return false
	}
	if h.model.QuickViewDisplayActive() {
		return false
	}
	if !h.carouselFilePreviewEligible() {
		return false
	}
	rect, ok := h.activePanelFileColumnRect()
	if !ok || !panelcarousel.LayoutFits(rect, h.model.CarouselLayout, true) {
		return false
	}
	if !panelcarousel.ShowChildPreviewColumn(*p, false, true) {
		return false
	}
	if panelcarousel.ChildPreviewKindFor(*p, false, true) != panelcarousel.ChildPreviewFile {
		return false
	}
	if h.model.Menu.Open || h.model.ModalDialogOpen() {
		return false
	}
	if h.model.ActiveSubFocus != ui.SubFocusFileList {
		return false
	}
	if h.model.ActivePanel != ui.PrimaryPanel && h.model.ActivePanel != ui.SecondaryPanel {
		return false
	}
	if _, ok := h.carouselFilePreviewWantPath(); !ok {
		return false
	}
	return true
}

func (h *Handler) applyCarouselFilePreviewNow() {
	if !h.carouselFilePreviewContext() {
		h.CloseCarouselFilePreview()
		return
	}
	path, ok := h.carouselFilePreviewWantPath()
	if !ok {
		h.CloseCarouselFilePreview()
		return
	}
	workDir := h.host.ActivePanel().PathString()
	err := localfs.CheckFilePreviewable(path)
	isImage := errors.Is(err, localfs.ErrFilePreviewImage)
	if err != nil && !isImage {
		switch {
		case errors.Is(err, localfs.ErrFilePreviewBinary):
			h.patchCarouselFilePreviewMessage(filepath.Base(path), "Not a text file")
		case errors.Is(err, localfs.ErrFilePreviewIsDir):
			h.patchCarouselFilePreviewMessage("", "Not a file")
		default:
			h.patchCarouselFilePreviewMessage(filepath.Base(path), err.Error())
		}
		return
	}
	tw, contentH, layOK := h.carouselChildPreviewLayoutMetrics()
	if !layOK {
		tw = 1
	}
	titleBase := filepath.Base(path)
	h.captureFilePreviewHold(previewTargetCarousel)
	h.patchCarouselFilePreview(func(st *ui.FilePreviewState) {
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
		// Keep ImagePayload* until the new encode finishes (stale-while-revalidate).
	})
	h.postRenderWake()
	gen := h.carouselFilePreviewRunGen.Add(1)
	go h.runPreview(h.ctx, h.previewRequest(path, tw, contentH, workDir, h.activePanelChromeBlocked(), h.gitStatusForPath(path), previewTargetCarousel, isImage), previewTargetCarousel, gen)
}

// refreshCarouselFilePreview re-runs the current carousel child preview at its current path,
// e.g. after a terminal resize changes the child column's text width. Scroll is left untouched
// (unlike applyCarouselFilePreviewNow, which is for opening/switching files).
func (h *Handler) refreshCarouselFilePreview() {
	h.mu.RLock()
	st := h.model.CarouselFilePreview
	h.mu.RUnlock()
	if !st.Open || st.Path == "" {
		return
	}
	tw, contentH, ok := h.carouselChildPreviewLayoutMetrics()
	if !ok {
		return
	}
	workDir := h.host.ActivePanel().PathString()
	req := h.previewRequest(st.Path, tw, contentH, workDir, h.activePanelChromeBlocked(), h.gitStatusForPath(st.Path), previewTargetCarousel, localfs.IsImagePath(st.Path))
	gen := h.carouselFilePreviewRunGen.Add(1)
	h.postRenderWake()
	go h.runPreview(h.ctx, req, previewTargetCarousel, gen)
}

// ReconcileCarouselFilePreview reapplies the carousel child-column file preview when its
// fingerprint has changed since the last apply. Called once per event from reconcileAfterEvent.
func (h *Handler) ReconcileCarouselFilePreview() {
	if h.model.Menu.Open || h.model.ModalDialogOpen() {
		return
	}
	p := h.host.ActivePanel()
	if !p.CarouselMode {
		// Active panel is not in carousel mode — do not touch the
		// global carousel file preview; the inactive panel (still with
		// CarouselMode=true) may be using it for its child column.
		return
	}
	eligible := h.carouselFilePreviewEligible()
	kind := panelcarousel.ChildPreviewKindFor(*p, h.model.QuickViewDisplayActive(), eligible)
	if kind != panelcarousel.ChildPreviewFile {
		if h.host.InQuickFilterUI() {
			// Preserve existing preview while user navigates with a filter active.
			return
		}
		h.mu.RLock()
		open := h.model.CarouselFilePreview.Open
		h.mu.RUnlock()
		if open || h.carouselFilePreviewLastFingerprint != "" {
			h.CloseCarouselFilePreview()
		}
		return
	}
	if h.model.ActiveSubFocus != ui.SubFocusFileList {
		return
	}
	sig := h.carouselFilePreviewFingerprint()
	if sig == h.carouselFilePreviewLastFingerprint {
		return
	}
	h.mu.RLock()
	previewOpen := h.model.CarouselFilePreview.Open
	h.mu.RUnlock()
	if !previewOpen {
		// Moving onto a file from a directory (or with nothing previewed yet) opens
		// the preview from closed. Debouncing here would blank the child column for
		// the debounce interval before the title paints, causing a visible flicker.
		// Apply immediately so the title renders in the same frame; the body still
		// loads asynchronously. File→file and directory navigation keep the existing
		// content visible meanwhile, so they debounce as before.
		h.ClearCarouselPreviewNavCoalesce()
		h.applyCarouselFilePreviewAfterFlush()
		return
	}
	if h.carouselPreviewNavSkipSnapshot.Load() {
		return
	}
	if h.host.Config().UI.KeyRepeatDebounceMS <= 0 {
		h.applyCarouselFilePreviewAfterFlush()
		return
	}
	h.ArmCarouselPreviewNavCoalesceAfterListNav()
}

func (h *Handler) applyCarouselFilePreviewAfterFlush() {
	if !h.carouselFilePreviewContext() {
		h.CloseCarouselFilePreview()
		return
	}
	h.applyCarouselFilePreviewNow()
	h.carouselFilePreviewLastFingerprint = h.carouselFilePreviewFingerprint()
}
