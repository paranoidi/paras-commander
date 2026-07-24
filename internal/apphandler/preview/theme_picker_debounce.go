package preview

import (
	"time"

	"github.com/gdamore/tcell/v2"
)

func (h *Handler) clearPreviewStylePickerDebounce() {
	h.previewStylePickerDebounce.Clear()
	h.previewStylePickerDebounceGen.Add(1)
}

func (h *Handler) schedulePreviewStylePickerDebounceTimer(gen uint64) {
	delay := time.Duration(h.host.Config().UI.KeyRepeatDebounceMS) * time.Millisecond
	h.previewStylePickerDebounce.Reset(delay, func() {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(StylePickerFlushPayload{gen: gen}))
	})
}

func (h *Handler) armPreviewStylePickerPreview(immediate bool) {
	if !h.model.FilePreviewThemePicker.Open {
		return
	}
	if h.filePreviewThemePickerSelectedName() == "" {
		return
	}
	if immediate || h.host.Config().UI.KeyRepeatDebounceMS <= 0 {
		h.flushPreviewStylePickerPreviewNow()
		return
	}
	gen := h.previewStylePickerDebounceGen.Add(1)
	h.schedulePreviewStylePickerDebounceTimer(gen)
}

func (h *Handler) flushPreviewStylePickerPreviewNow() {
	h.clearPreviewStylePickerDebounce()
	if !h.model.FilePreviewThemePicker.Open {
		return
	}
	if !h.syncPreviewStylePickerSelection() {
		return
	}
	h.refreshFullscreenFilePreview()
}

// ApplyPreviewStylePickerFlush applies the debounced style-picker preview reload. Returns true
// when a repaint is needed.
func (h *Handler) ApplyPreviewStylePickerFlush(p StylePickerFlushPayload) bool {
	if p.gen != h.previewStylePickerDebounceGen.Load() {
		return false
	}
	if !h.model.FilePreviewThemePicker.Open {
		return false
	}
	if !h.syncPreviewStylePickerSelection() {
		return false
	}
	h.refreshFullscreenFilePreview()
	return true
}

// FlushStylePickerPreviewNow applies the currently pending style-picker preview debounce
// immediately (skips waiting for the timer), for callers that need synchronous flush semantics.
func (h *Handler) FlushStylePickerPreviewNow() bool {
	return h.ApplyPreviewStylePickerFlush(StylePickerFlushPayload{gen: h.previewStylePickerDebounceGen.Load()})
}
