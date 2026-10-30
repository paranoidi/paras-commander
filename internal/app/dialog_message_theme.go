package app

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/filterlist"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func (a *App) openMessageDialog(title, message string) {
	a.model.MessageDialog = dialog.MessageDialogState{Open: true, Title: title, Message: message}
}

func (a *App) closeMessageDialog() {
	a.model.MessageDialog = dialog.MessageDialogState{}
}

// handleMessageDialogKey closes the dialog on the focused OK button (Enter, Space, Alt+O); every
// other key is swallowed.
func (a *App) handleMessageDialogKey(event *tcell.EventKey) {
	w, h := a.screen.Size()
	spec := dialog.MessageDialogView(a.model.MessageDialog, a.layoutForTerminalSize(w, h))
	focus := dlg.OK
	if ev, ok := dlg.HandleKey(spec, &focus, event); ok {
		a.applyMessageDialogEvent(ev)
	}
}

// applyMessageDialogEvent closes the message dialog on its OK button (key or click).
func (a *App) applyMessageDialogEvent(ev dlg.Event) {
	if ev.ID == dlg.OK {
		a.closeMessageDialog()
	}
}

func (a *App) handleThemeDialogKey(event *tcell.EventKey) {
	if event.Key() == tcell.KeyF5 {
		a.previewThemeAtSelection()
		return
	}
	td := &a.model.ThemeDialog
	if ev, ok := dlg.HandleKey(a.themeDialogView(), &td.Focus, event); ok {
		a.applyThemeDialogEvent(ev)
		return
	}
	if td.Focus != "" && td.Focus != dialog.ThemeFilter {
		return
	}
	w, h := a.screen.Size()
	width := max(dlg.InputTextWidth(w, h, a.themeDialogView(), dialog.ThemeFilter), 1)
	edit := scrollquery.NewEdit(&td.Query, &td.QueryCursor, &td.QueryScroll, width, func() {
		td.Selected = 0
		a.syncThemeDialogRanks()
		a.armThemeDialogPreview()
	})
	scrollquery.HandleKey(a.keys.DialogInput, event, true, edit)
}

// applyThemeDialogEvent acts on a widget event of the theme dialog (key or click).
func (a *App) applyThemeDialogEvent(ev dlg.Event) {
	td := &a.model.ThemeDialog
	switch {
	case ev.ID == "": // only focus moved
	case ev.Kind == dlg.EventCaret:
		td.QueryCursor = ev.Index
	case ev.Kind == dlg.EventListMove:
		td.Selected, td.ListScroll = ev.Index, ev.Offset
		a.armThemeDialogPreview()
	case ev.ID == dlg.Cancel:
		a.cancelThemeDialog()
	default: // Enter in the filter, a list click or OK
		a.activateThemeDialogSelection()
	}
}

func (a *App) themeDialogView() dlg.Spec {
	w, h := a.screen.Size()
	return dialog.ThemeDialogView(a.model.ThemeDialog, a.layoutForTerminalSize(w, h), a.styles)
}

// syncThemeDialogRanks re-ranks the theme choices against the filter query, clamps the selection
// and keeps it visible.
func (a *App) syncThemeDialogRanks() {
	td := &a.model.ThemeDialog
	lines := make([]string, len(td.Choices))
	for i, choice := range td.Choices {
		lines[i] = choice.Label
	}
	td.Ranked, td.MatchRanges = filterlist.SyncRanks(lines, td.Query, len(lines), a.config.Filter.CaseInsensitive)
	filterlist.ClampSelection(&td.Selected, len(td.Ranked))
	w, h := a.screen.Size()
	dialog.EnsureThemeListScroll(td, dialog.ThemeDialogListRows(*td, a.layoutForTerminalSize(w, h), a.styles))
}

func (a *App) openThemeDialog() {
	if len(a.model.ThemeDialog.Choices) == 0 {
		a.setTransientMessage("No themes available", ui.MessageUrgencyWarn)
		return
	}
	a.closeMessageDialog()
	a.themeAtDialogOpen = a.styles
	td := &a.model.ThemeDialog
	td.Query, td.QueryCursor, td.QueryScroll, td.ListScroll = "", 0, 0, 0
	td.Selected = a.currentThemeChoiceIndex() // an empty query ranks every choice in order
	td.Focus = dialog.ThemeFilter
	td.Open = true
	a.syncThemeDialogRanks()
	a.clearTransientMessage()
	a.previewThemeAtSelection()
}

func (a *App) closeThemeDialog() {
	a.themePreviewDebounce.Invalidate()
	a.model.ThemeDialog.Open = false
}

// cancelThemeDialog restores the theme active before the dialog was opened and closes it.
func (a *App) cancelThemeDialog() {
	a.closeThemeDialog()
	a.setStyles(a.themeAtDialogOpen)
}

// armThemeDialogPreview previews the selected theme once list motion settles: resolving and
// applying a theme repaints everything, so held arrow keys only load the theme they stop on.
func (a *App) armThemeDialogPreview() {
	ms := a.config.UI.KeyRepeatDebounceMS
	if ms <= 0 {
		a.previewThemeAtSelection()
		return
	}
	a.themePreviewDebounce.Arm(time.Duration(ms)*time.Millisecond, func() {
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(themePreviewFlushPayload{}))
	})
}

// themePreviewFlushPayload wakes the event loop to preview the theme dialog's settled selection.
type themePreviewFlushPayload struct{}

// themeDialogSelectedName is the theme under the dialog's cursor, or "" with no match.
func (a *App) themeDialogSelectedName() string {
	td := a.model.ThemeDialog
	if td.Selected < 0 || td.Selected >= len(td.Ranked) {
		return ""
	}
	ent := td.Ranked[td.Selected]
	if ent < 0 || ent >= len(td.Choices) {
		return ""
	}
	return td.Choices[ent].Name
}

func (a *App) previewThemeAtSelection() {
	a.themePreviewDebounce.Invalidate()
	if !a.model.ThemeDialog.Open {
		return
	}
	a.previewThemeByName(a.themeDialogSelectedName())
}

func (a *App) previewThemeByName(name string) {
	if name == "" {
		return
	}
	next, err := theme.Resolve(name, a.paths.ThemesDir)
	if err != nil {
		a.setTransientMessage(textutil.FirstLine(err.Error()), ui.MessageUrgencyCritical)
		if cached, ok := a.themes[name]; ok {
			a.setStyles(cached)
		}
		return
	}
	a.closeMessageDialog()
	a.clearTransientMessage()
	a.setStyles(next)
	a.themes[name] = next
}

func (a *App) activateThemeDialogSelection() {
	name := a.themeDialogSelectedName()
	if name == "" {
		a.cancelThemeDialog()
		return
	}
	a.themePreviewDebounce.Invalidate()
	if !a.applyTheme(name) {
		return
	}
	a.closeThemeDialog()
}

func (a *App) currentThemeChoiceIndex() int {
	currentName := a.model.ThemeDialog.CurrentName
	if currentName == "" {
		currentName = a.styles.Name
	}
	for index, choice := range a.model.ThemeDialog.Choices {
		if choice.Name == currentName {
			return index
		}
	}
	return 0
}

func (a *App) applyTheme(name string) bool {
	nextTheme, err := theme.Resolve(name, a.paths.ThemesDir)
	if err != nil {
		a.openMessageDialog("Theme failed", err.Error())
		return false
	}
	a.setStyles(nextTheme)
	a.themes[name] = nextTheme
	a.config.Theme = name
	a.model.ThemeDialog.CurrentName = name
	msg := fmt.Sprintf("Theme changed to %s", name)
	urgency := ui.MessageUrgencyInfo
	if err := a.persistPartial(map[string]interface{}{"theme": name}); err != nil {
		msg = fmt.Sprintf("%s (config save failed: %v)", msg, err)
		urgency = ui.MessageUrgencyWarn
	}
	a.setTransientMessage(msg, urgency)
	return true
}

func (a *App) persistPartial(patch map[string]interface{}) error {
	if !a.paths.CanPersist() {
		return nil
	}
	return config.WriteMergedPartial(a.paths, patch)
}

func uiThemeChoices(choices []theme.NamedTheme) []dialog.ThemeChoice {
	result := make([]dialog.ThemeChoice, 0, len(choices))
	for _, choice := range choices {
		result = append(result, dialog.ThemeChoice{Name: choice.Name, Label: choice.Label})
	}
	return result
}
