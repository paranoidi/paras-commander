package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) inactivePreviewChromeBlocked() bool {
	chromeBlocked := a.model.PanelsChromeBlocked()
	if a.inactivePanelID() == ui.LeftPanel && a.model.ThemeDialog.Open {
		return false
	}
	return chromeBlocked
}

func (a *App) activePanelChromeBlocked() bool {
	chromeBlocked := a.model.PanelsChromeBlocked()
	if a.model.ActivePanel == ui.LeftPanel && a.model.ThemeDialog.Open {
		return false
	}
	return chromeBlocked
}

func (a *App) fullscreenFilePreviewLayoutMetrics() (textW, contentH int, ok bool) {
	tw, ok := a.fullscreenPreviewTextWidth()
	if !ok {
		return 1, 0, false
	}
	union, ok := a.fullscreenPreviewUnionRect()
	if !ok {
		return tw, 0, false
	}
	preview, _ := ui.SplitFullscreenPreviewRects(union, a.model.FilePreviewThemePicker.Open, a.model.FilePreviewThemePicker.Choices)
	return tw, ui.JobsPanelContentRows(preview), true
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

func (a *App) snapshotPreviewDrawStates() {
	var inactive, fullscreen, carousel ui.FilePreviewState
	var warmInactive, warmFullscreen, warmCarousel bool
	var inactiveW, fullscreenW, carouselW int
	var inactiveBase, fullscreenBase, carouselBase tcell.Style

	a.commandsMu.RLock()
	inactive = a.model.FilePreview
	fullscreen = a.model.FullscreenFilePreview
	carousel = a.model.CarouselFilePreview
	a.commandsMu.RUnlock()

	inactive = ui.MergeFilePreviewDrawWithHold(inactive, a.filePreviewHold)
	a.overlayQuickViewInactiveDrawTitle(&inactive)
	fullscreen = ui.MergeFilePreviewDrawWithHold(fullscreen, a.fullscreenFilePreviewHold)
	carousel = ui.MergeFilePreviewDrawWithHold(carousel, a.carouselFilePreviewHold)

	if inactive.Open {
		tw, _, ok := a.inactivePanelPreviewLayoutMetrics(inactive.Open || a.model.QuickViewDisplayActive())
		if ok && filePreviewWarmCandidate(inactive) {
			warmInactive = true
			inactiveW = tw
			inactiveBase = ui.FilePreviewBodyStyle(a.styles, a.inactivePreviewChromeBlocked())
		}
	}
	if a.model.ViewMode == ui.ViewFilePreview && fullscreen.Open {
		tw, _, ok := a.fullscreenFilePreviewLayoutMetrics()
		if ok && filePreviewWarmCandidate(fullscreen) {
			warmFullscreen = true
			fullscreenW = tw
			fullscreenBase = ui.FilePreviewBodyStyle(a.styles, a.model.PanelsChromeBlocked())
		}
	}
	if carousel.Open {
		tw, _, ok := a.carouselChildPreviewLayoutMetrics()
		if ok && filePreviewWarmCandidate(carousel) {
			warmCarousel = true
			carouselW = tw
			carouselBase = ui.FilePreviewBodyStyle(a.styles, a.activePanelChromeBlocked())
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

	a.commandsMu.Lock()
	if warmInactive {
		a.model.FilePreview.WrapCacheSnapshot(inactive)
	}
	if warmFullscreen {
		a.model.FullscreenFilePreview.WrapCacheSnapshot(fullscreen)
	}
	if warmCarousel {
		a.model.CarouselFilePreview.WrapCacheSnapshot(carousel)
	}
	a.model.FilePreviewDraw = inactive
	a.model.CarouselFilePreviewDraw = carousel
	a.model.FullscreenFilePreviewDraw = fullscreen
	a.commandsMu.Unlock()
}

func (a *App) filePreviewLineCount(textW int) int {
	base := ui.FilePreviewBodyStyle(a.styles, a.inactivePreviewChromeBlocked())

	a.commandsMu.RLock()
	st := a.model.FilePreview
	a.commandsMu.RUnlock()

	if count, ok := st.CachedWrappedLineCount(textW); ok {
		return count
	}

	warmed := warmFilePreviewCopy(st, textW, base)

	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	if previewBodyCacheMatches(a.model.FilePreview, warmed) {
		a.model.FilePreview.WrapCacheSnapshot(warmed)
		if count, ok := a.model.FilePreview.CachedWrappedLineCount(textW); ok {
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

func (a *App) fullscreenFilePreviewLineCount(textW int) int {
	base := ui.FilePreviewBodyStyle(a.styles, a.model.PanelsChromeBlocked())

	a.commandsMu.RLock()
	st := a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()

	if count, ok := st.CachedWrappedLineCount(textW); ok {
		return count
	}

	warmed := warmFilePreviewCopy(st, textW, base)

	a.commandsMu.Lock()
	defer a.commandsMu.Unlock()
	if previewBodyCacheMatches(a.model.FullscreenFilePreview, warmed) {
		a.model.FullscreenFilePreview.WrapCacheSnapshot(warmed)
		if count, ok := a.model.FullscreenFilePreview.CachedWrappedLineCount(textW); ok {
			return count
		}
	}
	return warmed.WrappedLineCount(textW, base)
}

// overlayQuickViewInactiveDrawTitle aligns the inactive-column draw snapshot title with the
// driver's current file selection so the top-row filename updates during nav coalesce
// without waiting for the debounced preview reload.
func (a *App) overlayQuickViewInactiveDrawTitle(st *ui.FilePreviewState) {
	if !a.model.QuickViewDisplayActive() || a.model.QuickViewDirOverlayActive {
		return
	}
	path, _, mode := a.quickViewWantFile()
	if mode != quickViewWantFile {
		return
	}
	st.Open = true
	if tb := filepath.Base(path); tb != "" && tb != "." {
		st.TitleBase = tb
	}
}
