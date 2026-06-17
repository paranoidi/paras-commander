package app

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

// previewStylePickerFlushPayload re-runs F3 preview highlighting after style-picker debounce.
type previewStylePickerFlushPayload struct {
	gen uint64
}

func (a *App) clearPreviewStylePickerDebounce() {
	a.previewStylePickerDebounceMu.Lock()
	if a.previewStylePickerDebounceTimer != nil {
		if !a.previewStylePickerDebounceTimer.Stop() {
			select {
			case <-a.previewStylePickerDebounceTimer.C:
			default:
			}
		}
		a.previewStylePickerDebounceTimer = nil
	}
	a.previewStylePickerDebounceMu.Unlock()
	a.previewStylePickerDebounceGen.Add(1)
}

func (a *App) schedulePreviewStylePickerDebounceTimer(gen uint64) {
	delay := time.Duration(a.config.Preview.StylePickerDebounceMS) * time.Millisecond
	a.previewStylePickerDebounceMu.Lock()
	defer a.previewStylePickerDebounceMu.Unlock()
	if a.previewStylePickerDebounceTimer != nil {
		if !a.previewStylePickerDebounceTimer.Stop() {
			select {
			case <-a.previewStylePickerDebounceTimer.C:
			default:
			}
		}
		a.previewStylePickerDebounceTimer = nil
	}
	a.previewStylePickerDebounceTimer = time.AfterFunc(delay, func() {
		a.previewStylePickerDebounceMu.Lock()
		a.previewStylePickerDebounceTimer = nil
		a.previewStylePickerDebounceMu.Unlock()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(previewStylePickerFlushPayload{gen: gen}))
	})
}

func (a *App) armPreviewStylePickerPreview(immediate bool) {
	if !a.model.FilePreviewThemePicker.Open {
		return
	}
	if !a.syncPreviewStylePickerSelection() {
		return
	}
	if immediate || a.config.Preview.StylePickerDebounceMS <= 0 {
		a.flushPreviewStylePickerPreviewNow()
		return
	}
	gen := a.previewStylePickerDebounceGen.Add(1)
	a.schedulePreviewStylePickerDebounceTimer(gen)
}

func (a *App) flushPreviewStylePickerPreviewNow() {
	a.clearPreviewStylePickerDebounce()
	if !a.model.FilePreviewThemePicker.Open {
		return
	}
	if !a.syncPreviewStylePickerSelection() {
		return
	}
	a.refreshFullscreenFilePreview()
}

func (a *App) applyPreviewStylePickerFlush(p previewStylePickerFlushPayload) bool {
	if p.gen != a.previewStylePickerDebounceGen.Load() {
		return false
	}
	if !a.model.FilePreviewThemePicker.Open {
		return false
	}
	if !a.syncPreviewStylePickerSelection() {
		return false
	}
	a.refreshFullscreenFilePreview()
	return true
}
