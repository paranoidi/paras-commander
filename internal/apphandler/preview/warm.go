package preview

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (h *Handler) inactivePreviewChromeBlocked() bool {
	chromeBlocked := h.model.PanelsChromeBlocked()
	if h.host.InactivePanelID() == ui.PrimaryPanel && h.model.ThemeDialog.Open {
		return false
	}
	return chromeBlocked
}

func (h *Handler) activePanelChromeBlocked() bool {
	chromeBlocked := h.model.PanelsChromeBlocked()
	if h.model.ActivePanel == ui.PrimaryPanel && h.model.ThemeDialog.Open {
		return false
	}
	return chromeBlocked
}

func (h *Handler) fullscreenFilePreviewLayoutMetrics() (textW, contentH int, ok bool) {
	tw, ok := h.fullscreenPreviewTextWidth()
	if !ok {
		return 1, 0, false
	}
	union, ok := h.fullscreenPreviewUnionRect()
	if !ok {
		return tw, 0, false
	}
	previewRect, _ := ui.SplitFullscreenPreviewRects(union, h.model.FilePreviewThemePicker.Open, h.model.FilePreviewThemePicker.Choices)
	// Borderless: only the filename row is reserved (no top+bottom border), so subtract 1, not 2.
	contentH = previewRect.Height - 1
	if h.model.FullscreenFilePreview.Search.Editing {
		contentH--
	}
	if contentH < 0 {
		contentH = 0
	}
	return tw, contentH, true
}

func filePreviewWarmCandidate(st ui.FilePreviewState) bool {
	return ui.FilePreviewDrawWarmCandidate(st)
}

func warmFilePreviewCopy(st ui.FilePreviewState, textW int, base tcell.Style) ui.FilePreviewState {
	if !filePreviewWarmCandidate(st) {
		return st
	}
	st.EnsureWrappedLines(textW, base)
	return st
}

// SnapshotPreviewDrawStates builds the draw-ready (stale-while-revalidate) snapshots for all
// three preview panes into the model's *Draw fields. Called once per render from render.go.
func (h *Handler) SnapshotPreviewDrawStates() {
	var inactive, fullscreen, carousel ui.FilePreviewState
	var warmInactive, warmFullscreen, warmCarousel bool
	var inactiveW, fullscreenW, carouselW int
	var inactiveBase, fullscreenBase, carouselBase tcell.Style

	h.mu.RLock()
	inactive = h.model.FilePreview
	fullscreen = h.model.FullscreenFilePreview
	carousel = h.model.CarouselFilePreview
	h.mu.RUnlock()

	inactive = ui.MergeFilePreviewDrawWithHold(inactive, h.filePreviewHold)
	h.overlayQuickViewInactiveDrawTitle(&inactive)

	// During a folder→file debounce transition, keep the dir overlay visible until
	// the file preview has actual content to display (Phase done or hold content arrived).
	if h.model.QuickViewDirOverlayVisualHold {
		if inactive.Open && (inactive.Phase == ui.FilePreviewPhaseDone || inactive.BodyHeld) {
			h.clearQuickViewDirOverlayVisualHold()
		} else {
			// File content not yet ready — suppress the loading chrome.
			inactive = ui.FilePreviewState{}
		}
	}
	fullscreen = ui.MergeFilePreviewDrawWithHold(fullscreen, h.fullscreenFilePreviewHold)
	carousel = ui.MergeFilePreviewDrawWithHold(carousel, h.carouselFilePreviewHold)

	if inactive.Open {
		tw, _, ok := h.inactivePanelPreviewLayoutMetrics(inactive.Open || h.model.QuickViewDisplayActive())
		if ok && filePreviewWarmCandidate(inactive) {
			warmInactive = true
			inactiveW = tw
			inactiveBase = ui.FilePreviewBodyStyle(h.host.Styles(), h.inactivePreviewChromeBlocked())
		}
	}
	if h.model.ViewMode == ui.ViewFilePreview && fullscreen.Open {
		tw, _, ok := h.fullscreenFilePreviewLayoutMetrics()
		if ok && filePreviewWarmCandidate(fullscreen) {
			warmFullscreen = true
			fullscreenW = tw
			fullscreenBase = ui.FilePreviewBodyStyle(h.host.Styles(), h.model.PanelsChromeBlocked())
		}
	}
	if carousel.Open {
		tw, _, ok := h.carouselChildPreviewLayoutMetrics()
		if ok && filePreviewWarmCandidate(carousel) {
			warmCarousel = true
			carouselW = tw
			carouselBase = ui.FilePreviewBodyStyle(h.host.Styles(), h.activePanelChromeBlocked())
		}
	}

	if warmInactive {
		inactive = warmFilePreviewCopy(inactive, inactiveW, inactiveBase)
	}
	if warmFullscreen {
		fullscreen = warmFilePreviewCopy(fullscreen, fullscreenW, fullscreenBase)
	}
	if warmCarousel {
		carousel = warmFilePreviewCopy(carousel, carouselW, carouselBase)
	}

	h.mu.Lock()
	if warmInactive {
		h.model.FilePreview.WrapCacheSnapshot(inactive)
	}
	if warmFullscreen {
		h.model.FullscreenFilePreview.WrapCacheSnapshot(fullscreen)
	}
	if warmCarousel {
		h.model.CarouselFilePreview.WrapCacheSnapshot(carousel)
	}
	h.model.FilePreviewDraw = inactive
	h.model.CarouselFilePreviewDraw = carousel
	h.model.FullscreenFilePreviewDraw = fullscreen
	h.mu.Unlock()
}

func (h *Handler) filePreviewLineCount(textW int) int {
	base := ui.FilePreviewBodyStyle(h.host.Styles(), h.inactivePreviewChromeBlocked())

	h.mu.RLock()
	st := h.model.FilePreview
	h.mu.RUnlock()

	if count, ok := st.CachedWrappedLineCount(textW); ok {
		return count
	}

	warmed := warmFilePreviewCopy(st, textW, base)

	h.mu.Lock()
	defer h.mu.Unlock()
	if previewBodyCacheMatches(h.model.FilePreview, warmed) {
		h.model.FilePreview.WrapCacheSnapshot(warmed)
		if count, ok := h.model.FilePreview.CachedWrappedLineCount(textW); ok {
			return count
		}
	}
	return warmed.WrappedLineCount(textW, base)
}

func previewBodyCacheMatches(live, warmed ui.FilePreviewState) bool {
	if live.Source != warmed.Source {
		return false
	}
	switch live.Source {
	case ui.PreviewSourceInternalHighlighted:
		if len(live.HighlightedCells) != len(warmed.HighlightedCells) {
			return false
		}
		return live.HighlightedCacheKey() == warmed.HighlightedCacheKey()
	default:
		return live.CombinedText == warmed.CombinedText
	}
}

func (h *Handler) carouselFilePreviewLineCount(textW int) int {
	base := ui.FilePreviewBodyStyle(h.host.Styles(), h.activePanelChromeBlocked())

	h.mu.RLock()
	st := h.model.CarouselFilePreview
	h.mu.RUnlock()

	if count, ok := st.CachedWrappedLineCount(textW); ok {
		return count
	}

	warmed := warmFilePreviewCopy(st, textW, base)

	h.mu.Lock()
	defer h.mu.Unlock()
	if previewBodyCacheMatches(h.model.CarouselFilePreview, warmed) {
		h.model.CarouselFilePreview.WrapCacheSnapshot(warmed)
		if count, ok := h.model.CarouselFilePreview.CachedWrappedLineCount(textW); ok {
			return count
		}
	}
	return warmed.WrappedLineCount(textW, base)
}

func (h *Handler) fullscreenFilePreviewLineCount(textW int) int {
	base := ui.FilePreviewBodyStyle(h.host.Styles(), h.model.PanelsChromeBlocked())

	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()

	if count, ok := st.CachedWrappedLineCount(textW); ok {
		return count
	}

	warmed := warmFilePreviewCopy(st, textW, base)

	h.mu.Lock()
	defer h.mu.Unlock()
	if previewBodyCacheMatches(h.model.FullscreenFilePreview, warmed) {
		h.model.FullscreenFilePreview.WrapCacheSnapshot(warmed)
		if count, ok := h.model.FullscreenFilePreview.CachedWrappedLineCount(textW); ok {
			return count
		}
	}
	return warmed.WrappedLineCount(textW, base)
}

// overlayQuickViewInactiveDrawTitle aligns the inactive-column draw snapshot title with the
// driver's current file selection so the top-row filename updates during nav coalesce
// without waiting for the debounced preview reload.
func (h *Handler) overlayQuickViewInactiveDrawTitle(st *ui.FilePreviewState) {
	if !h.model.QuickViewDisplayActive() || h.model.QuickViewDirOverlayActive || h.model.QuickViewDirOverlayVisualHold {
		return
	}
	path, _, mode := h.quickViewWantFile()
	if mode != quickViewWantFile {
		return
	}
	st.Open = true
	if tb := filepath.Base(path); tb != "" && tb != "." {
		st.TitleBase = tb
	}
}
