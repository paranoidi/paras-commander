package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// carouselPreviewFlushPayload reloads the carousel child side preview after file-list debounce.
type carouselPreviewFlushPayload struct {
	gen uint64
}

func (a *App) clearCarouselPreviewDebounce() {
	a.carouselPreviewDebounceMu.Lock()
	if a.carouselPreviewDebounceTimer != nil {
		if !a.carouselPreviewDebounceTimer.Stop() {
			select {
			case <-a.carouselPreviewDebounceTimer.C:
			default:
			}
		}
		a.carouselPreviewDebounceTimer = nil
	}
	a.carouselPreviewDebounceMu.Unlock()
	a.carouselPreviewDebounceGen.Add(1)
	a.carouselPreviewNavSkipSnapshot.Store(false)
}

// clearCarouselPreviewNavCoalesce stops pending carousel side-preview coalesce.
func (a *App) clearCarouselPreviewNavCoalesce() {
	a.clearCarouselPreviewDebounce()
}

func (a *App) carouselPreviewNavCoalesceContext() bool {
	return a.model.ViewMode == ui.ViewBrowser &&
		a.activePanel().CarouselMode &&
		a.activePanel().CarouselCenterHasSubdirectories() &&
		a.model.ActiveSubFocus == ui.SubFocusFileList &&
		!a.model.Menu.Open &&
		!a.model.ModalDialogOpen() &&
		!a.inQuickFilterUI()
}

func (a *App) scheduleCarouselPreviewDebounceTimer(gen uint64) {
	delay := time.Duration(a.config.UI.CarouselPreviewDebounceMS) * time.Millisecond
	a.carouselPreviewDebounceMu.Lock()
	defer a.carouselPreviewDebounceMu.Unlock()
	if a.carouselPreviewDebounceTimer != nil {
		if !a.carouselPreviewDebounceTimer.Stop() {
			select {
			case <-a.carouselPreviewDebounceTimer.C:
			default:
			}
		}
		a.carouselPreviewDebounceTimer = nil
	}
	a.carouselPreviewDebounceTimer = time.AfterFunc(delay, func() {
		a.carouselPreviewDebounceMu.Lock()
		a.carouselPreviewDebounceTimer = nil
		a.carouselPreviewDebounceMu.Unlock()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(carouselPreviewFlushPayload{gen: gen}))
	})
}

func (a *App) armCarouselPreviewNavCoalesceAfterListNav() {
	if a.config.UI.CarouselPreviewDebounceMS <= 0 {
		return
	}
	if !a.carouselPreviewNavCoalesceContext() {
		return
	}
	a.carouselPreviewNavSkipSnapshot.Store(true)
	gen := a.carouselPreviewDebounceGen.Add(1)
	a.scheduleCarouselPreviewDebounceTimer(gen)
}

func (a *App) applyCarouselPreviewFlush(p carouselPreviewFlushPayload) bool {
	if p.gen != a.carouselPreviewDebounceGen.Load() {
		return false
	}
	a.carouselPreviewNavSkipSnapshot.Store(false)
	a.loadCarouselChildPreviewFromDisk()
	return true
}

// loadCarouselChildPreviewFromDisk reads the child preview listing after nav coalesce ends.
func (a *App) loadCarouselChildPreviewFromDisk() {
	if !a.carouselPreviewNavCoalesceContext() {
		return
	}
	p := a.activePanel()
	p.CarouselChildPreviewCoalesce = false
	_, _ = p.SnapshotChild(a.activeViewportRows())
}

// carouselPreviewHeldListNav reports file-list nav keys while carousel child preview coalesce may apply.
func (a *App) carouselPreviewHeldListNav(resolvedAction string, event *tcell.EventKey) bool {
	if a.config.UI.CarouselPreviewDebounceMS <= 0 || !a.activePanel().CarouselMode {
		return false
	}
	return a.panelSyncFollowHeldListNav(resolvedAction, event)
}

// syncCarouselChildPreviewCoalesceFlags sets child-preview coalesce before painting carousel columns.
func (a *App) syncCarouselChildPreviewCoalesceFlags() {
	coalesce := a.carouselPreviewNavSkipSnapshot.Load() && a.carouselPreviewNavCoalesceContext()
	a.model.Left.CarouselChildPreviewCoalesce = coalesce && a.model.Left.CarouselMode && a.model.ActivePanel == ui.LeftPanel
	a.model.Right.CarouselChildPreviewCoalesce = coalesce && a.model.Right.CarouselMode && a.model.ActivePanel == ui.RightPanel
}
