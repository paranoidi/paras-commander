package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func chromaStyleUIChoices() []dialog.ThemeChoice {
	src := chromastyles.Choices()
	out := make([]dialog.ThemeChoice, len(src))
	for i, c := range src {
		out[i] = dialog.ThemeChoice{Name: c.Name, Label: c.Label}
	}
	return out
}

func (a *App) toggleFilePreviewThemePicker() {
	if a.model.ViewMode != ui.ViewFilePreview || !a.model.FullscreenFilePreview.Open {
		return
	}
	if a.model.FilePreviewThemePicker.Open {
		return
	}
	a.openFilePreviewThemePicker()
}

func (a *App) openFilePreviewThemePicker() {
	a.clearPreviewStylePickerDebounce()
	choices := chromaStyleUIChoices()
	if len(choices) == 0 {
		a.setTransientMessage("No Chroma styles available", ui.MessageUrgencyWarn)
		return
	}
	a.previewStyleAtPickerOpen = a.config.Preview.Style
	display := make([]string, len(choices))
	for i, c := range choices {
		display[i] = c.Label
	}
	a.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{
		Open:         true,
		Choices:      choices,
		DisplayLines: display,
	}
	a.syncFilePreviewThemePickerRanks()
	curIdx := chromastyles.IndexOf(chromastyles.Choices(), a.config.Preview.Style)
	selected := 0
	for i, idx := range a.model.FilePreviewThemePicker.Ranked {
		if idx == curIdx {
			selected = i
			break
		}
	}
	a.model.FilePreviewThemePicker.Selected = selected
	a.armPreviewStylePickerPreview(true)
	rect := a.filePreviewThemePickerRect()
	dialog.EnsureFilePreviewThemePickerListScroll(&a.model.FilePreviewThemePicker, ui.FilePreviewThemePickerListRows(rect))
}

func (a *App) closeFilePreviewThemePicker(revert bool) {
	a.clearPreviewStylePickerDebounce()
	if revert {
		a.config.Preview.Style = a.previewStyleAtPickerOpen
		a.refreshFullscreenFilePreview()
	}
	a.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{}
}

func (a *App) applyFilePreviewThemePickerSelection() {
	st := &a.model.FilePreviewThemePicker
	if !st.Open {
		return
	}
	name := a.filePreviewThemePickerSelectedName()
	if name == "" {
		a.closeFilePreviewThemePicker(false)
		return
	}
	a.flushPreviewStylePickerPreviewNow()
	if a.applyPreviewStyle(name) {
		a.closeFilePreviewThemePicker(false)
	}
}

func (a *App) filePreviewThemePickerSelectedName() string {
	st := &a.model.FilePreviewThemePicker
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return ""
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Choices) {
		return ""
	}
	return st.Choices[entIdx].Name
}

func (a *App) syncPreviewStylePickerSelection() bool {
	name := a.filePreviewThemePickerSelectedName()
	if name == "" {
		return false
	}
	a.config.Preview.Style = config.NormalizePreviewStyle(name)
	return true
}

func (a *App) previewFilePreviewThemePickerSelection() {
	a.armPreviewStylePickerPreview(false)
}

func (a *App) applyPreviewStyle(name string) bool {
	validated := config.NormalizePreviewStyle(name)
	a.config.Preview.Style = validated
	msg := fmt.Sprintf("Preview style changed to %s", validated)
	urgency := ui.MessageUrgencyInfo
	if err := a.persistPartial(map[string]interface{}{
		"preview": map[string]interface{}{
			"style": validated,
		},
	}); err != nil {
		msg = fmt.Sprintf("%s (config save failed: %v)", msg, err)
		urgency = ui.MessageUrgencyWarn
	}
	a.setTransientMessage(msg, urgency)
	return true
}

func (a *App) refreshFullscreenFilePreview() {
	a.commandsMu.RLock()
	st := a.model.FullscreenFilePreview
	a.commandsMu.RUnlock()
	if !st.Open || st.Path == "" {
		return
	}
	tw, ok := a.fullscreenPreviewTextWidth()
	if !ok {
		return
	}
	gen := a.filePreviewRunGen.Add(1)
	a.postCommandWake()
	go a.runPreview(a.commandsCtx, a.previewRequest(st.Path, tw, a.activePanel().PathString(), a.model.PanelsChromeBlocked(), a.gitStatusForPath(st.Path)), previewTargetFullscreen, gen)
}

func (a *App) syncFilePreviewThemePickerRanks() {
	st := &a.model.FilePreviewThemePicker
	if !st.Open {
		return
	}
	lines := filePreviewThemePickerDisplayLines(st)
	st.DisplayLines = lines
	st.Ranked, st.MatchRanges = syncFilteredListRanks(lines, st.Query, len(lines), a.config.CaseInsensitiveFilter)
	clampFilteredListSelection(&st.Selected, len(st.Ranked))
	rect := a.filePreviewThemePickerRect()
	dialog.EnsureFilePreviewThemePickerListScroll(st, ui.FilePreviewThemePickerListRows(rect))
}

func (a *App) filePreviewThemePickerRect() ui.Rect {
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		return ui.Rect{}
	}
	union := ui.MergeTwinPanelRects(lay.Primary, lay.Secondary, a.effectivePaneSplitOrientation())
	_, picker := ui.SplitFullscreenPreviewRects(union, true, a.model.FilePreviewThemePicker.Choices)
	return picker
}

func (a *App) filePreviewThemePickerListRows() int {
	return ui.FilePreviewThemePickerListRows(a.filePreviewThemePickerRect())
}

func (a *App) filePreviewThemePickerQueryWidth() int {
	return ui.FilePreviewThemePickerQueryWidth(a.filePreviewThemePickerRect())
}

func filePreviewThemePickerDisplayLines(st *dialog.FilePreviewThemePickerState) []string {
	lines := make([]string, len(st.Choices))
	for i, choice := range st.Choices {
		switch {
		case choice.Label != "":
			lines[i] = choice.Label
		case i < len(st.DisplayLines) && st.DisplayLines[i] != "":
			lines[i] = st.DisplayLines[i]
		default:
			lines[i] = choice.Name
		}
	}
	return lines
}

func filePreviewThemePickerScrollingQuery(st *dialog.FilePreviewThemePickerState, width int, onChange func()) scrollingQueryEdit {
	return newScrollingQueryEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, width, onChange)
}

func (a *App) handleFilePreviewThemePickerKey(event *tcell.EventKey) bool {
	st := &a.model.FilePreviewThemePicker
	if !st.Open {
		return false
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeFilePreviewThemePicker(true)
		return true
	case tcell.KeyEnter:
		a.applyFilePreviewThemePickerSelection()
		return true
	}

	onChange := func() {
		a.syncFilePreviewThemePickerRanks()
		st.Selected = 0
		a.previewFilePreviewThemePickerSelection()
		dialog.EnsureFilePreviewThemePickerListScroll(st, a.filePreviewThemePickerListRows())
	}
	if a.handleScrollingQueryKey(event, true, filePreviewThemePickerScrollingQuery(st, a.filePreviewThemePickerQueryWidth(), onChange)) {
		return true
	}

	ensureScroll := func() {
		dialog.EnsureFilePreviewThemePickerListScroll(st, a.filePreviewThemePickerListRows())
	}
	if handleFilteredListSelectionKey(event, 0, &st.Selected, len(st.Ranked), a.filePreviewThemePickerListRows, ensureScroll) {
		a.previewFilePreviewThemePickerSelection()
		return true
	}

	switch event.Key() {
	case tcell.KeyHome:
		if len(st.Ranked) > 0 {
			st.Selected = 0
			ensureScroll()
			a.previewFilePreviewThemePickerSelection()
		}
		return true
	case tcell.KeyEnd:
		if len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			ensureScroll()
			a.previewFilePreviewThemePickerSelection()
		}
		return true
	}

	return true
}

func (a *App) fullscreenPreviewUnionRect() (ui.Rect, bool) {
	w, h := a.screen.Size()
	lay := a.layoutForTerminalSize(w, h)
	if lay.TooSmall {
		return ui.Rect{}, false
	}
	return ui.MergeTwinPanelRects(lay.Primary, lay.Secondary, a.effectivePaneSplitOrientation()), true
}

func (a *App) fullscreenPreviewTextWidth() (int, bool) {
	union, ok := a.fullscreenPreviewUnionRect()
	if !ok {
		return 1, false
	}
	preview, _ := ui.SplitFullscreenPreviewRects(union, a.model.FilePreviewThemePicker.Open, a.model.FilePreviewThemePicker.Choices)
	tw := preview.Width - 4
	if tw < 1 {
		tw = 1
	}
	return tw, true
}
