package preview

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// startFilePreviewSearch begins an incremental "/" search in the fullscreen preview.
func (h *Handler) startFilePreviewSearch() {
	if !h.model.FullscreenFilePreview.Open {
		return
	}
	if h.model.FullscreenFilePreview.ImagePayload != "" {
		return
	}
	h.clearFilePreviewSearchField()
	h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.StartSearch()
		st.Search.MatchStyle = h.host.Styles().FuzzyHighlight
		st.Search.CurrentStyle = h.host.Styles().FuzzyHighlightCursor
	})
}

func (h *Handler) clearFilePreviewSearchField() {
	h.mu.Lock()
	h.model.FullscreenFilePreviewSearchField = dialog.FileDialogField{}
	h.mu.Unlock()
}

func (h *Handler) applyFilePreviewSearchFieldEdit(field dialog.FileDialogField) {
	h.mu.Lock()
	h.model.FullscreenFilePreviewSearchField = field
	query := field.Value
	h.mu.Unlock()
	h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Search.Query = query
		st.RecomputeSearch()
	})
	h.mu.RLock()
	current := h.model.FullscreenFilePreview.Search.Current
	h.mu.RUnlock()
	h.scrollFilePreviewToSearchMatch(current)
}

// handleFilePreviewSearchTypingKey captures keystrokes while a preview search query is
// being edited (Search.Editing). All runes, including ones that would otherwise be bound
// to actions like "n"/"p", are literal query text here.
func (h *Handler) handleFilePreviewSearchTypingKey(event *tcell.EventKey) (quit bool) {
	switch event.Key() {
	case tcell.KeyEsc:
		h.clearFilePreviewSearchField()
		h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) { st.CancelSearch() })
		return false
	case tcell.KeyEnter:
		h.mu.RLock()
		query := h.model.FullscreenFilePreviewSearchField.Value
		h.mu.RUnlock()
		h.clearFilePreviewSearchField()
		h.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
			st.Search.Query = query
			st.AcceptSearch()
		})
		return false
	}

	h.mu.RLock()
	field := h.model.FullscreenFilePreviewSearchField
	h.mu.RUnlock()

	switch event.Key() {
	case tcell.KeyCtrlU, tcell.KeyCtrlL:
		if field.Value != "" {
			field.Clear()
			h.applyFilePreviewSearchFieldEdit(field)
		}
		return false
	}

	if h.host.HandleFileDialogFieldKey(event, &field, func() {
		h.applyFilePreviewSearchFieldEdit(field)
	}) {
		return false
	}
	return false
}

// filePreviewSearchNav moves to the next (dir>0) or previous (dir<0) search match, wrapping
// at the ends, and scrolls the preview to reveal it.
func (h *Handler) filePreviewSearchNav(dir int) {
	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()
	if !st.Search.Active || len(st.Search.Matches) == 0 {
		return
	}
	n := len(st.Search.Matches)
	next := (st.Search.Current + dir + n) % n
	h.patchFullscreenFilePreview(func(fp *ui.FilePreviewState) { fp.Search.Current = next })
	h.scrollFilePreviewToSearchMatch(next)
}

// scrollFilePreviewToSearchMatch scrolls the fullscreen preview so search match idx is visible.
func (h *Handler) scrollFilePreviewToSearchMatch(idx int) {
	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()
	if idx < 0 || idx >= len(st.Search.Matches) {
		return
	}
	tw, ok := h.previewTextWidth(previewTargetFullscreen)
	if !ok || tw < 1 {
		return
	}
	offset := st.SourceLineToScrollOffset(st.Search.Matches[idx].Line, tw, tcell.StyleDefault)
	h.hunkScrollTo(previewTargetFullscreen, offset)
}
