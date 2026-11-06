package preview

import (
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

// HandleFullscreenSelectionMouse drives text selection in the fullscreen preview: a left press on
// the text area starts it, dragging extends it (scrolling one row per report past the top or
// bottom edge) and the release returns the text to copy. A doubleClick press selects the word under
// the pointer and returns it at once, so a stray drag before the release cannot shrink it. handled
// is false for reports that are not part of a selection.
func (h *Handler) HandleFullscreenSelectionMouse(x, y int, pressed, doubleClick, held bool) (text string, handled bool) {
	textW, contentH, ok := h.fullscreenFilePreviewLayoutMetrics()
	rect, rectOK := h.fullscreenPreviewRect()
	if !ok || !rectOK || contentH <= 0 {
		return "", false
	}
	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()
	textX, top := rect.X, rect.Y+1
	if st.IsMarkdown {
		textX++
	}
	base := ui.FilePreviewBodyStyle(h.host.Styles(), h.model.PanelsChromeBlocked())
	switch {
	case pressed:
		if !st.TextSelectable() || x < textX || x >= textX+textW || y < top || y >= top+contentH {
			return "", false
		}
		if doubleClick {
			word := st.WordSelectionAt(st.Scroll+y-top, x-textX, textW, base)
			h.patchFullscreenFilePreview(func(s *ui.FilePreviewState) { s.Selection = word })
			st.Selection = word
			return st.SelectedText(), true
		}
		off := st.CellOffsetAt(st.Scroll+y-top, x-textX, textW, base)
		h.patchFullscreenFilePreview(func(s *ui.FilePreviewState) {
			s.Selection = previewpanel.Selection{Anchor: off, Head: off, Dragging: true}
		})
		return "", true
	case !st.Selection.Dragging:
		return "", false
	case held:
		if y < top {
			h.fullscreenPreviewScrollBy(-1)
		} else if y >= top+contentH {
			h.fullscreenPreviewScrollBy(1)
		}
		h.mu.RLock()
		st = h.model.FullscreenFilePreview
		h.mu.RUnlock()
		row := max(0, min(y-top, contentH-1))
		off := st.CellOffsetAt(st.Scroll+row, max(0, x-textX), textW, base)
		h.patchFullscreenFilePreview(func(s *ui.FilePreviewState) { s.Selection.Head = off })
		return "", true
	}
	h.patchFullscreenFilePreview(func(s *ui.FilePreviewState) { s.Selection.Dragging = false })
	return st.SelectedText(), true
}
