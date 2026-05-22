package app

import (
	"github.com/paranoidi/paras-commander/internal/app/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// syncPathFieldCompletion updates filesystem completion ghost text on a path input field.
func (a *App) syncPathFieldCompletion(f *ui.FileDialogField, textWidth int) {
	if f == nil {
		return
	}
	if f.Prefill != "" && f.PrefillPending && f.Value == f.Prefill {
		f.ClearCompletion()
		a.syncPathFieldScroll(f, textWidth)
		return
	}
	panel := a.activePanel()
	c, ok := pathpick.SuggestAtCursor(panel.PathString(), a.model.UserHomeDir, f.Value, f.Cursor, a.config.ShowHidden)
	if !ok {
		f.ClearCompletion()
		a.syncPathFieldScroll(f, textWidth)
		return
	}
	f.CompletionSuffix = c.Suffix
	f.CompletionIsDir = c.IsDir
	a.syncPathFieldScroll(f, textWidth)
}

func (a *App) syncPathFieldScroll(f *ui.FileDialogField, textWidth int) {
	if f == nil || textWidth <= 0 {
		return
	}
	valueLen := len([]rune(f.Value))
	suffixLen := len([]rune(f.CompletionSuffix))
	f.Cursor, f.Scroll = ui.EnsurePathInputScroll(valueLen, f.Cursor, f.Scroll, textWidth, suffixLen)
}

// syncOpenPathInputsAfterFSChange refreshes filesystem completion on open path fields
// after the directory listing may have changed (panel refresh, validation tick, etc.).
func (a *App) syncOpenPathInputsAfterFSChange() {
	if a.model.PathPicker.Open {
		a.syncPathPickerCompletion()
	}
	d := &a.model.TransferDialog
	if d.Open && d.Phase == ui.TransferPhaseDestination {
		a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
	}
}

func (a *App) transferDestinationTextWidth() int {
	termW, _ := a.screen.Size()
	frameW := ui.PreferredFormDialogWidth
	if frameW > termW-4 {
		frameW = termW - 4
	}
	if frameW < 36 {
		frameW = 36
	}
	return frameW - 4 - 2
}
