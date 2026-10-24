package app

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/config"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/textutil"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func (a *App) openMessageDialog(title, message string) {
	a.model.MessageDialog.Title = title
	a.model.MessageDialog.Message = message
	a.model.MessageDialog.TwoButtons = false
	a.model.MessageDialog.ButtonFocus = 0
	a.model.MessageDialog.Open = true
}

func (a *App) closeMessageDialog() {
	a.model.MessageDialog.Open = false
	a.model.MessageDialog.Title = ""
	a.model.MessageDialog.Message = ""
	a.model.MessageDialog.TwoButtons = false
	a.model.MessageDialog.ButtonFocus = 0
}

func (a *App) handleMessageDialogKey(event *tcell.EventKey) {
	d := &a.model.MessageDialog
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		return
	}
	if d.TwoButtons {
		switch event.Key() {
		case tcell.KeyLeft:
			if d.ButtonFocus > 0 {
				d.ButtonFocus--
			}
			return
		case tcell.KeyRight:
			if d.ButtonFocus < 1 {
				d.ButtonFocus++
			}
			return
		case tcell.KeyEnter:
			a.closeMessageDialog()
			return
		default:
			return
		}
	}
	if event.Key() == tcell.KeyEnter {
		a.closeMessageDialog()
	}
}

func (a *App) handleThemeDialogKey(event *tcell.EventKey) {
	if event.Key() == tcell.KeyF5 {
		a.previewThemeAtSelection()
		return
	}
	td := &a.model.ThemeDialog
	ev, ok := dlg.HandleKey(a.themeDialogView(), &td.Focus, event)
	switch {
	case !ok || ev.ID == "": // not handled, or only focus moved
	case ev.Kind == dlg.EventListMove:
		td.Selected = ev.Index
		a.previewThemeAtSelection()
	case ev.ID == dlg.Cancel:
		a.cancelThemeDialog()
	default: // list or OK
		a.activateThemeDialogSelection()
	}
}

func (a *App) themeDialogView() dlg.Spec {
	w, h := a.screen.Size()
	return dialog.ThemeDialogView(a.model.ThemeDialog, a.layoutForTerminalSize(w, h))
}

func (a *App) openThemeDialog() {
	if len(a.model.ThemeDialog.Choices) == 0 {
		a.setTransientMessage("No themes available", ui.MessageUrgencyWarn)
		return
	}
	a.closeMessageDialog()
	a.themeAtDialogOpen = a.styles
	a.model.ThemeDialog.Selected = a.currentThemeChoiceIndex()
	a.model.ThemeDialog.Focus = dialog.ThemeList
	a.model.ThemeDialog.Open = true
	a.clearTransientMessage()
	a.previewThemeAtSelection()
}

func (a *App) closeThemeDialog() {
	a.model.ThemeDialog.Open = false
}

// cancelThemeDialog restores the theme active before the dialog was opened and closes it.
func (a *App) cancelThemeDialog() {
	a.setStyles(a.themeAtDialogOpen)
	a.closeThemeDialog()
}

func (a *App) previewThemeAtSelection() {
	if !a.model.ThemeDialog.Open {
		return
	}
	choices := a.model.ThemeDialog.Choices
	sel := a.model.ThemeDialog.Selected
	if sel < 0 || sel >= len(choices) {
		return
	}
	a.previewThemeByName(choices[sel].Name)
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
	choices := a.model.ThemeDialog.Choices
	selected := a.model.ThemeDialog.Selected
	if selected < 0 || selected >= len(choices) {
		a.closeThemeDialog()
		return
	}
	if !a.applyTheme(choices[selected].Name) {
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
