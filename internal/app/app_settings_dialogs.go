package app

import (
	"fmt"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Sort dialog handlers

func (a *App) openSortDialog() {
	a.openSortDialogForPanel(a.model.ActivePanel)
}

func (a *App) openSortDialogForPanel(panelID int) {
	target := a.panelByID(panelID)
	a.model.SortDialog = ui.SortDialogState{
		Open:                  true,
		SortMode:              target.Sort.Mode,
		SortReverse:           target.Sort.Reverse,
		DirectoriesFirst:      target.Sort.DirectoriesFirst,
		DiskUsageIdleSizeSort: target.Sort.DiskUsageIdleSizeSort,
		Focus:                 0,
		PanelID:               panelID,
	}
}

func (a *App) closeSortDialog() {
	a.model.SortDialog.Open = false
}

func (a *App) applySortDialog() {
	target := a.panelByID(a.model.SortDialog.PanelID)
	target.ApplySortFromDialog(panel.SortState{
		Mode:                  a.model.SortDialog.SortMode,
		Reverse:               a.model.SortDialog.SortReverse,
		DirectoriesFirst:      a.model.SortDialog.DirectoriesFirst,
		DiskUsageIdleSizeSort: a.model.SortDialog.DiskUsageIdleSizeSort,
	}, a.panelViewportRows(a.model.SortDialog.PanelID))
	a.setTransientMessage(fmt.Sprintf("Sort: %s", target.Sort.Mode.String()), ui.MessageUrgencyInfo)
	a.closeSortDialog()
}

func (a *App) handleSortDialogKey(event *tcell.EventKey) {
	form := ui.NewDialogLinearForm(7)
	// Alt+O = OK, Alt+C = Cancel
	if ui.AltDialogOK(event) {
		a.applySortDialog()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeSortDialog()
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeSortDialog()
	case tcell.KeyEnter:
		switch a.model.SortDialog.Focus {
		case form.CancelIndex():
			a.closeSortDialog()
		default: // OK button or any radio/checkbox -> apply
			a.applySortDialog()
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch event.Rune() {
		case 'n', 'N':
			a.model.SortDialog.SortMode = panel.SortName
			a.model.SortDialog.Focus = 0
		case 'e', 'E':
			a.model.SortDialog.SortMode = panel.SortExtension
			a.model.SortDialog.Focus = 1
		case 's', 'S':
			a.model.SortDialog.SortMode = panel.SortSize
			a.model.SortDialog.Focus = 2
		case 'm', 'M':
			a.model.SortDialog.SortMode = panel.SortMtime
			a.model.SortDialog.Focus = 3
		case 'u', 'U':
			a.model.SortDialog.DiskUsageIdleSizeSort = !a.model.SortDialog.DiskUsageIdleSizeSort
			a.model.SortDialog.Focus = 4
		case 'r', 'R':
			a.model.SortDialog.SortReverse = !a.model.SortDialog.SortReverse
			a.model.SortDialog.Focus = 5
		case 'd', 'D':
			a.model.SortDialog.DirectoriesFirst = !a.model.SortDialog.DirectoriesFirst
			a.model.SortDialog.Focus = 6
		case 'o', 'O':
			a.applySortDialog()
		case 'c', 'C':
			a.closeSortDialog()
		case ' ':
			switch a.model.SortDialog.Focus {
			case 0, 1, 2, 3:
				modes := []panel.SortMode{panel.SortName, panel.SortExtension, panel.SortSize, panel.SortMtime}
				a.model.SortDialog.SortMode = modes[a.model.SortDialog.Focus]
			case 4:
				a.model.SortDialog.DiskUsageIdleSizeSort = !a.model.SortDialog.DiskUsageIdleSizeSort
			case 5:
				a.model.SortDialog.SortReverse = !a.model.SortDialog.SortReverse
			case 6:
				a.model.SortDialog.DirectoriesFirst = !a.model.SortDialog.DirectoriesFirst
			case 7:
				a.applySortDialog()
			case 8:
				a.closeSortDialog()
			}
		}
	}
	if focus, ok := form.MoveFocus(a.model.SortDialog.Focus, event.Key()); ok {
		a.model.SortDialog.Focus = focus
	}
}

func (a *App) openConfigDialog() {
	a.clearTransientMessage()
	a.model.ConfigDialog = ui.ConfigDialogState{
		Open:          true,
		ShowFileIcons: a.config.UI.ShowFileIcons,
		Focus:         0,
	}
}

func (a *App) closeConfigDialog() {
	a.model.ConfigDialog.Open = false
}

func (a *App) applyConfigDialog() {
	val := a.model.ConfigDialog.ShowFileIcons
	a.config.UI.ShowFileIcons = val
	a.model.ShowFileIcons = val
	a.closeConfigDialog()
	msg := "Configuration saved"
	patch := map[string]interface{}{
		"ui": map[string]interface{}{
			"show_file_icons": val,
		},
	}
	if err := a.persistPartial(patch); err != nil {
		msg = fmt.Sprintf("Configuration saved (could not write config: %v)", err)
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}

func (a *App) handleConfigDialogKey(event *tcell.EventKey) {
	form := ui.NewDialogLinearForm(1)
	if ui.AltDialogOK(event) {
		a.applyConfigDialog()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeConfigDialog()
		return
	}
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeConfigDialog()
	case tcell.KeyEnter:
		switch a.model.ConfigDialog.Focus {
		case form.CancelIndex():
			a.closeConfigDialog()
		default:
			a.applyConfigDialog()
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		switch event.Rune() {
		case 'f', 'F':
			a.model.ConfigDialog.ShowFileIcons = !a.model.ConfigDialog.ShowFileIcons
			a.model.ConfigDialog.Focus = 0
		case 'o', 'O':
			a.applyConfigDialog()
		case 'c', 'C':
			a.closeConfigDialog()
		case ' ':
			switch a.model.ConfigDialog.Focus {
			case 0:
				a.model.ConfigDialog.ShowFileIcons = !a.model.ConfigDialog.ShowFileIcons
			case form.OKIndex():
				a.applyConfigDialog()
			case form.CancelIndex():
				a.closeConfigDialog()
			}
		}
	}
	if focus, ok := form.MoveFocus(a.model.ConfigDialog.Focus, event.Key()); ok {
		a.model.ConfigDialog.Focus = focus
	}
}

// Group selection dialog handlers

func (a *App) openGroupSelect(mode string) {
	a.model.GroupSelect = ui.GroupSelectState{
		Open:             true,
		Text:             "",
		Mode:             mode,
		FilesOnly:        false,
		CaseSensitive:    false,
		UseShellPatterns: true,
		Focus:            0,
	}
}

func (a *App) closeGroupSelect() {
	a.model.GroupSelect.Open = false
	a.model.GroupSelect.Text = ""
}

func (a *App) executeGroupSelect() {
	gs := &a.model.GroupSelect
	if gs.Text == "" {
		return
	}
	p := a.activePanel()
	if gs.Mode == "select" {
		p.SelectGroup(gs.Text, gs.FilesOnly, gs.CaseSensitive, gs.UseShellPatterns)
		a.setTransientMessage(fmt.Sprintf("Selected matching %q", gs.Text), ui.MessageUrgencyInfo)
	} else {
		p.UnselectGroup(gs.Text, gs.FilesOnly, gs.CaseSensitive, gs.UseShellPatterns)
		a.setTransientMessage(fmt.Sprintf("Unselected matching %q", gs.Text), ui.MessageUrgencyInfo)
	}
	a.closeGroupSelect()
}

func (a *App) handleGroupSelectKey(event *tcell.EventKey) {
	gs := &a.model.GroupSelect
	form := ui.NewDialogLinearForm(4)
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeGroupSelect()
	case tcell.KeyEnter:
		switch gs.Focus {
		case 5: // Cancel
			a.closeGroupSelect()
		default: // pattern input, checkboxes, or OK -> execute
			a.executeGroupSelect()
		}
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if gs.Focus == 0 {
			runes := []rune(gs.Text)
			if len(runes) > 0 {
				gs.Text = string(runes[:len(runes)-1])
			}
		}
	case tcell.KeyRune:
		// Mnemonics follow dialog standards: Alt+letter only (plain typing goes into the pattern).
		if event.Modifiers() == tcell.ModAlt {
			switch event.Rune() {
			case 'o', 'O':
				a.executeGroupSelect()
			case 'c', 'C':
				a.closeGroupSelect()
			case 'f', 'F':
				gs.FilesOnly = !gs.FilesOnly
				gs.Focus = 1
			case 's', 'S':
				gs.CaseSensitive = !gs.CaseSensitive
				gs.Focus = 2
			case 'u', 'U':
				gs.UseShellPatterns = !gs.UseShellPatterns
				gs.Focus = 3
			}
			break
		}
		mod := event.Modifiers()
		if mod != tcell.ModNone && mod != tcell.ModShift {
			break
		}
		if event.Rune() == ' ' {
			if gs.Focus == 0 {
				gs.Text += " "
				break
			}
			switch gs.Focus {
			case 1:
				gs.FilesOnly = !gs.FilesOnly
			case 2:
				gs.CaseSensitive = !gs.CaseSensitive
			case 3:
				gs.UseShellPatterns = !gs.UseShellPatterns
			case 4:
				a.executeGroupSelect()
			case 5:
				a.closeGroupSelect()
			}
			break
		}
		if gs.Focus == 0 && unicode.IsPrint(event.Rune()) {
			gs.Text += string(event.Rune())
		}
	}
	if focus, ok := form.MoveFocus(gs.Focus, event.Key()); ok {
		gs.Focus = focus
	}
}
