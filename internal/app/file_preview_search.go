package app

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// startFilePreviewSearch begins an incremental "/" search in the fullscreen preview.
func (a *App) startFilePreviewSearch() {
	if !a.model.FullscreenFilePreview.Open {
		return
	}
	a.clearFilePreviewSearchField()
	a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.StartSearch()
		st.Search.MatchStyle = a.styles.FuzzyHighlight
		st.Search.CurrentStyle = a.styles.FuzzyHighlightCursor
	})
}

func (a *App) clearFilePreviewSearchField() {
	a.commandsMu.Lock()
	a.model.FullscreenFilePreviewSearchField = dialog.FileDialogField{}
	a.commandsMu.Unlock()
}

func (a *App) applyFilePreviewSearchFieldEdit(field dialog.FileDialogField) {
	a.commandsMu.Lock()
	a.model.FullscreenFilePreviewSearchField = field
	query := field.Value
	a.commandsMu.Unlock()
	a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
		st.Search.Query = query
		st.RecomputeSearch()
	})
	a.commandsMu.RLock()
	current := a.model.FullscreenFilePreview.Search.Current
	a.commandsMu.RUnlock()
	a.scrollFilePreviewToSearchMatch(current)
}

// handleFilePreviewSearchTypingKey captures keystrokes while a preview search query is
// being edited (Search.Editing). All runes, including ones that would otherwise be bound
// to actions like "n"/"p", are literal query text here.
func (a *App) handleFilePreviewSearchTypingKey(event *tcell.EventKey) (quit bool) {
	switch event.Key() {
	case tcell.KeyEsc:
		a.clearFilePreviewSearchField()
		a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) { st.CancelSearch() })
		return false
	case tcell.KeyEnter:
		a.commandsMu.RLock()
		query := a.model.FullscreenFilePreviewSearchField.Value
		a.commandsMu.RUnlock()
		a.clearFilePreviewSearchField()
		a.patchFullscreenFilePreview(func(st *ui.FilePreviewState) {
			st.Search.Query = query
			st.AcceptSearch()
		})
		return false
	}

	a.commandsMu.RLock()
	field := a.model.FullscreenFilePreviewSearchField
	a.commandsMu.RUnlock()

	switch event.Key() {
	case tcell.KeyCtrlU, tcell.KeyCtrlL:
		if field.Value != "" {
			field.Clear()
			a.applyFilePreviewSearchFieldEdit(field)
		}
		return false
	}

	if a.handleFileDialogFieldKey(event, &field, func() {
		a.applyFilePreviewSearchFieldEdit(field)
	}) {
		return false
	}
	return false
}

// filePreviewSearchNav moves to the next (dir>0) or previous (dir<0) search match, wrapping
// at the ends, and scrolls the preview to reveal it.
func (a *App) filePreviewSearchNav(dir int) {
	a.commandsMu.RLock()
	st := a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()
	if !st.Search.Active || len(st.Search.Matches) == 0 {
		return
	}
	n := len(st.Search.Matches)
	next := (st.Search.Current + dir + n) % n
	a.patchFullscreenFilePreview(func(fp *ui.FilePreviewState) { fp.Search.Current = next })
	a.scrollFilePreviewToSearchMatch(next)
}

// scrollFilePreviewToSearchMatch scrolls the fullscreen preview so search match idx is visible.
func (a *App) scrollFilePreviewToSearchMatch(idx int) {
	a.commandsMu.RLock()
	st := a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()
	if idx < 0 || idx >= len(st.Search.Matches) {
		return
	}
	tw, ok := a.previewTextWidth(previewTargetFullscreen)
	if !ok || tw < 1 {
		return
	}
	offset := st.SourceLineToScrollOffset(st.Search.Matches[idx].Line, tw, tcell.StyleDefault)
	a.hunkScrollTo(previewTargetFullscreen, offset)
}
