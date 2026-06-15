package app

import (
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) captureFilePreviewHold(target previewTarget) {
	a.commandsMu.RLock()
	var from ui.FilePreviewState
	switch target {
	case previewTargetFullscreen:
		from = a.model.FullscreenFilePreview
	case previewTargetCarousel:
		from = a.model.CarouselFilePreview
	default:
		from = a.model.FilePreview
	}
	a.commandsMu.RUnlock()
	if !ui.FilePreviewHoldable(from) {
		return
	}
	hold := from
	switch target {
	case previewTargetFullscreen:
		a.fullscreenFilePreviewHold = hold
	case previewTargetCarousel:
		a.carouselFilePreviewHold = hold
	default:
		a.filePreviewHold = hold
	}
}

func (a *App) clearFilePreviewHold(target previewTarget) {
	switch target {
	case previewTargetFullscreen:
		a.fullscreenFilePreviewHold = ui.FilePreviewState{}
	case previewTargetCarousel:
		a.carouselFilePreviewHold = ui.FilePreviewState{}
	default:
		a.filePreviewHold = ui.FilePreviewState{}
	}
}
