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
	a.previewStylePickerDebounce.Clear()
	a.previewStylePickerDebounceGen.Add(1)
}

func (a *App) schedulePreviewStylePickerDebounceTimer(gen uint64) {
	delay := time.Duration(a.config.UI.KeyRepeatDebounceMS) * time.Millisecond
	a.previewStylePickerDebounce.Reset(delay, func() {
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
	if immediate || a.config.UI.KeyRepeatDebounceMS <= 0 {
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
