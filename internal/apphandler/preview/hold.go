package preview

import (
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (h *Handler) captureFilePreviewHold(target previewTarget) {
	h.mu.RLock()
	var from ui.FilePreviewState
	switch target {
	case previewTargetFullscreen:
		from = h.model.FullscreenFilePreview
	case previewTargetCarousel:
		from = h.model.CarouselFilePreview
	default:
		from = h.model.FilePreview
	}
	h.mu.RUnlock()
	if !ui.FilePreviewHoldable(from) {
		return
	}
	hold := from
	switch target {
	case previewTargetFullscreen:
		h.fullscreenFilePreviewHold = hold
	case previewTargetCarousel:
		h.carouselFilePreviewHold = hold
	default:
		h.filePreviewHold = hold
	}
}

func (h *Handler) clearFilePreviewHold(target previewTarget) {
	switch target {
	case previewTargetFullscreen:
		h.fullscreenFilePreviewHold = ui.FilePreviewState{}
	case previewTargetCarousel:
		h.carouselFilePreviewHold = ui.FilePreviewState{}
	default:
		h.filePreviewHold = ui.FilePreviewState{}
	}
}
