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
		if dialog.AltDialogOK(event) {
			a.closeMessageDialog()
			return
		}
		if dialog.AltDialogCancel(event) && d.TwoButtons {
			a.closeMessageDialog()
			return
		}
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
		case tcell.KeyEsc:
			a.closeMessageDialog()
			return
		case tcell.KeyEnter:
			a.closeMessageDialog()
			return
		default:
			return
		}
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyEnter:
		a.closeMessageDialog()
	}
}

func (a *App) handleThemeDialogKey(event *tcell.EventKey) {
	// Alt+O = OK, Alt+C = Cancel
	if dialog.AltDialogOK(event) {
		a.activateThemeDialogSelection()
		return
	}
	if dialog.AltDialogCancel(event) {
		a.cancelThemeDialog()
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.cancelThemeDialog()
	case tcell.KeyF5:
		a.previewThemeAtSelection()
	case tcell.KeyEnter:
		switch a.model.ThemeDialog.Focus {
		case 2: // Cancel
			a.cancelThemeDialog()
		default: // list or OK
			a.activateThemeDialogSelection()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		td := &a.model.ThemeDialog
		if nf, ok := dialog.ListOKCancelNavFocusKey(td.Focus, event.Key()); ok {
			td.Focus = nf
			break
		}
		if td.Focus == 0 && event.Key() == tcell.KeyUp {
			a.moveThemeDialog(-1)
			break
		}
		if td.Focus == 0 && event.Key() == tcell.KeyDown {
			a.moveThemeDialog(1)
			break
		}
	case tcell.KeyHome:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.model.ThemeDialog.Selected = 0
			a.previewThemeAtSelection()
		}
	case tcell.KeyEnd:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.model.ThemeDialog.Selected = len(a.model.ThemeDialog.Choices) - 1
			a.previewThemeAtSelection()
		}
	case tcell.KeyPgUp:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.moveThemeDialog(-a.themeDialogListViewportRows())
		}
	case tcell.KeyPgDn:
		if a.model.ThemeDialog.Focus == 0 && len(a.model.ThemeDialog.Choices) > 0 {
			a.moveThemeDialog(a.themeDialogListViewportRows())
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch dialog.DialogButtonRune(event.Rune()) {
		case dialog.ButtonRuneOK:
			a.activateThemeDialogSelection()
		case dialog.ButtonRuneCancel:
			a.cancelThemeDialog()
		case dialog.ButtonRuneToggle:
			switch a.model.ThemeDialog.Focus {
			case 0:
				a.activateThemeDialogSelection()
			case 1:
				a.activateThemeDialogSelection()
			case 2:
				a.cancelThemeDialog()
			}
		}
	}
}

func (a *App) openThemeDialog() {
	if len(a.model.ThemeDialog.Choices) == 0 {
		a.setTransientMessage("No themes available", ui.MessageUrgencyWarn)
		return
	}
	a.closeMessageDialog()
	a.themeAtDialogOpen = a.styles
	a.model.ThemeDialog.Selected = a.currentThemeChoiceIndex()
	a.model.ThemeDialog.Focus = 0
	a.model.ThemeDialog.Open = true
	a.clearTransientMessage()
	a.previewThemeAtSelection()
}

func (a *App) closeThemeDialog() {
	a.model.ThemeDialog.Open = false
}

// cancelThemeDialog restores the theme active before the dialog was opened and closes it.
func (a *App) cancelThemeDialog() {
	a.styles = a.themeAtDialogOpen
	a.closeThemeDialog()
}

func (a *App) moveThemeDialog(delta int) {
	count := len(a.model.ThemeDialog.Choices)
	if count == 0 {
		a.model.ThemeDialog.Selected = 0
		return
	}
	a.model.ThemeDialog.Selected = wrap(a.model.ThemeDialog.Selected+delta, count)
	a.previewThemeAtSelection()
}

func (a *App) themeDialogListViewportRows() int {
	w, h := a.screen.Size()
	layout := a.layoutForTerminalSize(w, h)
	if layout.TooSmall {
		return 1
	}
	return dialog.ThemeDialogListViewportRows(layout, len(a.model.ThemeDialog.Choices))
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
			a.styles = cached
		}
		return
	}
	a.closeMessageDialog()
	a.clearTransientMessage()
	a.styles = next
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
	a.styles = nextTheme
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
