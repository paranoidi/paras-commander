package preview

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	previewrun "github.com/paranoidi/paras-commander/internal/preview"
	"github.com/paranoidi/paras-commander/internal/preview/chromastyles"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
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

func (h *Handler) toggleFilePreviewThemePicker() {
	if h.model.ViewMode != ui.ViewFilePreview || !h.model.FullscreenFilePreview.Open {
		return
	}
	if h.model.FullscreenFilePreview.ImagePayload != "" {
		return
	}
	if h.model.FilePreviewThemePicker.Open {
		return
	}
	h.openFilePreviewThemePicker()
}

func (h *Handler) openFilePreviewThemePicker() {
	h.clearPreviewStylePickerDebounce()
	choices := chromaStyleUIChoices()
	if len(choices) == 0 {
		h.host.SetTransientMessage("No Chroma styles available", ui.MessageUrgencyWarn)
		return
	}
	h.previewStyleAtPickerOpen = h.host.Config().Preview.Style
	display := make([]string, len(choices))
	for i, c := range choices {
		display[i] = c.Label
	}
	h.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{
		Open:         true,
		Choices:      choices,
		DisplayLines: display,
	}
	h.syncFilePreviewThemePickerRanks()
	curIdx := chromastyles.IndexOf(chromastyles.Choices(), h.host.Config().Preview.Style)
	selected := 0
	for i, idx := range h.model.FilePreviewThemePicker.Ranked {
		if idx == curIdx {
			selected = i
			break
		}
	}
	h.model.FilePreviewThemePicker.Selected = selected
	h.armPreviewStylePickerPreview(true)
	rect := h.filePreviewThemePickerRect()
	dialog.EnsureFilePreviewThemePickerListScroll(&h.model.FilePreviewThemePicker, ui.FilePreviewThemePickerListRows(rect))
}

func (h *Handler) closeFilePreviewThemePicker(revert bool) {
	h.clearPreviewStylePickerDebounce()
	if revert {
		h.host.SetPreviewStyle(h.previewStyleAtPickerOpen)
		h.refreshFullscreenFilePreview()
	}
	h.model.FilePreviewThemePicker = dialog.FilePreviewThemePickerState{}
}

func (h *Handler) applyFilePreviewThemePickerSelection() {
	st := &h.model.FilePreviewThemePicker
	if !st.Open {
		return
	}
	name := h.filePreviewThemePickerSelectedName()
	if name == "" {
		h.closeFilePreviewThemePicker(false)
		return
	}
	h.flushPreviewStylePickerPreviewNow()
	if h.host.ApplyPreviewStyle(name) {
		h.closeFilePreviewThemePicker(false)
	}
}

// SelectedPreviewStyleName returns the Chroma style name currently highlighted in the F3 style
// picker, or "" when the picker is closed or has no selection.
func (h *Handler) SelectedPreviewStyleName() string {
	return h.filePreviewThemePickerSelectedName()
}

func (h *Handler) filePreviewThemePickerSelectedName() string {
	st := &h.model.FilePreviewThemePicker
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return ""
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Choices) {
		return ""
	}
	return st.Choices[entIdx].Name
}

func (h *Handler) syncPreviewStylePickerSelection() bool {
	name := h.filePreviewThemePickerSelectedName()
	if name == "" {
		return false
	}
	style := config.NormalizePreviewStyle(name)
	if style == h.host.Config().Preview.Style {
		return false
	}
	h.host.SetPreviewStyle(style)
	return true
}

func (h *Handler) previewFilePreviewThemePickerSelection() {
	h.armPreviewStylePickerPreview(false)
}

func (h *Handler) refreshFullscreenFilePreview() {
	h.mu.RLock()
	st := h.model.FullscreenFilePreview
	h.mu.RUnlock()
	if !st.Open || st.Path == "" {
		return
	}
	tw, contentH, ok := h.fullscreenFilePreviewLayoutMetrics()
	if !ok {
		return
	}
	req := h.previewRequest(st.Path, tw, contentH, h.host.ActivePanel().PathString(), h.model.PanelsChromeBlocked(), h.gitStatusForPath(st.Path), previewTargetFullscreen)
	req.RawMarkdown = h.model.FullscreenFilePreviewRawMarkdown
	gen := h.filePreviewRunGen.Add(1)
	h.postRenderWake()
	go h.runPreview(h.ctx, req, previewTargetFullscreen, gen)
}

func (h *Handler) syncFilePreviewThemePickerRanks() {
	st := &h.model.FilePreviewThemePicker
	if !st.Open {
		return
	}
	lines := filePreviewThemePickerDisplayLines(st)
	st.DisplayLines = lines
	st.Ranked, st.MatchRanges = h.host.SyncFilteredListRanks(lines, st.Query, len(lines), h.host.Config().Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&st.Selected, len(st.Ranked))
	rect := h.filePreviewThemePickerRect()
	dialog.EnsureFilePreviewThemePickerListScroll(st, ui.FilePreviewThemePickerListRows(rect))
}

func (h *Handler) filePreviewThemePickerRect() ui.Rect {
	w, ht := h.screen.Size()
	lay := h.host.LayoutForTerminalSize(w, ht)
	if lay.TooSmall {
		return ui.Rect{}
	}
	union := ui.MergeTwinPanelRects(lay.Primary, lay.Secondary, h.host.EffectivePaneSplitOrientation())
	_, picker := ui.SplitFullscreenPreviewRects(union, true, h.model.FilePreviewThemePicker.Choices)
	return picker
}

func (h *Handler) filePreviewThemePickerListRows() int {
	return ui.FilePreviewThemePickerListRows(h.filePreviewThemePickerRect())
}

func (h *Handler) filePreviewThemePickerQueryWidth() int {
	return ui.FilePreviewThemePickerQueryWidth(h.filePreviewThemePickerRect())
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

func (h *Handler) handleFilePreviewThemePickerKey(event *tcell.EventKey) bool {
	st := &h.model.FilePreviewThemePicker
	if !st.Open {
		return false
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.closeFilePreviewThemePicker(true)
		return true
	case tcell.KeyEnter:
		h.applyFilePreviewThemePickerSelection()
		return true
	}

	onChange := func() {
		h.syncFilePreviewThemePickerRanks()
		st.Selected = 0
		h.previewFilePreviewThemePickerSelection()
		dialog.EnsureFilePreviewThemePickerListScroll(st, h.filePreviewThemePickerListRows())
	}
	edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, h.filePreviewThemePickerQueryWidth(), onChange)
	if scrollquery.HandleKey(h.keysDialogInput, event, true, edit) {
		return true
	}

	ensureScroll := func() {
		dialog.EnsureFilePreviewThemePickerListScroll(st, h.filePreviewThemePickerListRows())
	}
	if h.host.HandleFilteredListSelectionKey(event, 0, &st.Selected, len(st.Ranked), h.filePreviewThemePickerListRows, ensureScroll) {
		h.previewFilePreviewThemePickerSelection()
		return true
	}

	switch event.Key() {
	case tcell.KeyHome:
		if len(st.Ranked) > 0 {
			st.Selected = 0
			ensureScroll()
			h.previewFilePreviewThemePickerSelection()
		}
		return true
	case tcell.KeyEnd:
		if len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			ensureScroll()
			h.previewFilePreviewThemePickerSelection()
		}
		return true
	}

	return true
}

func (h *Handler) fullscreenPreviewUnionRect() (ui.Rect, bool) {
	w, ht := h.screen.Size()
	// Fullscreen preview reclaims the menu row (borderless, filename on row 0), so compute
	// the layout with the menu bar unreserved to match ui.Render.
	lay := ui.CalculateLayoutWithOrientation(w, ht, false, h.host.PanelPaneSplit(w, true), h.host.EffectivePaneSplitOrientation(), h.host.TerminalLayoutRows(), 0)
	if lay.TooSmall {
		return ui.Rect{}, false
	}
	return ui.MergeTwinPanelRects(lay.Primary, lay.Secondary, h.host.EffectivePaneSplitOrientation()), true
}

func (h *Handler) fullscreenPreviewTextWidth() (int, bool) {
	union, ok := h.fullscreenPreviewUnionRect()
	if !ok {
		return 1, false
	}
	previewRect, _ := ui.SplitFullscreenPreviewRects(union, h.model.FilePreviewThemePicker.Open, h.model.FilePreviewThemePicker.Choices)
	tw := previewRect.Width // borderless: no side border columns, only a right-side scrollbar gutter
	if h.fullscreenPreviewRendersMarkdown() {
		tw -= 2 // 1-space left margin + 1-space right margin/scrollbar gutter for rendered markdown
	} else {
		tw-- // 1-space right scrollbar gutter
	}
	if tw < 1 {
		tw = 1
	}
	return tw, true
}

// fullscreenPreviewRendersMarkdown reports whether the fullscreen preview's next content will be
// produced by the rendered-markdown formatter, mirroring preview.Run's dispatch (including the
// git-diff-to-plain-content fallback) so layout can reserve the markdown margin ahead of the
// async run completing.
func (h *Handler) fullscreenPreviewRendersMarkdown() bool {
	h.mu.RLock()
	path := h.model.FullscreenFilePreview.Path
	h.mu.RUnlock()
	if path == "" {
		return false
	}
	req := previewrun.Request{
		Path:        path,
		Preview:     h.host.Config().Preview,
		RawMarkdown: h.model.FullscreenFilePreviewRawMarkdown,
	}
	if gitStatus := h.gitStatusForPath(path); gitStatus != nil {
		req.GitDiff = true
		req.GitStatus = gitStatus
	}
	return previewrun.WillRenderMarkdown(req)
}
