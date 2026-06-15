package app

import (
	"strings"

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
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		return 1, 0, false
	}
	union := ui.MergeTwinPanelRects(lay.Left, lay.Right)
	tw := union.Width - 4
	if tw < 1 {
		tw = 1
	}
	return tw, ui.JobsPanelContentRows(union), true
}

func filePreviewWarmCandidate(st ui.FilePreviewState) bool {
	if !st.Open || st.Phase != ui.FilePreviewPhaseDone || strings.TrimSpace(st.ErrorMsg) != "" {
		return false
	}
	if strings.TrimSpace(st.CombinedText) == "" && st.ExitCode == 0 {
		return false
	}
	return true
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
	a.model.FilePreviewDraw = a.model.FilePreview
	a.model.CarouselFilePreviewDraw = a.model.CarouselFilePreview
	a.model.FullscreenFilePreviewDraw = a.model.FullscreenFilePreview
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
	if a.model.FilePreview.CombinedText == warmed.CombinedText {
		a.model.FilePreview.WrapCacheSnapshot(warmed)
		if count, ok := a.model.FilePreview.CachedWrappedLineCount(textW); ok {
			return count
		}
	}
	return warmed.WrappedLineCount(textW, base)
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
	if a.model.FullscreenFilePreview.CombinedText == warmed.CombinedText {
		a.model.FullscreenFilePreview.WrapCacheSnapshot(warmed)
		if count, ok := a.model.FullscreenFilePreview.CachedWrappedLineCount(textW); ok {
			return count
		}
	}
	return warmed.WrappedLineCount(textW, base)
}
