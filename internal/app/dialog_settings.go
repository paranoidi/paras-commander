package app

import (
	"fmt"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
)

// Sort dialog handlers

func (a *App) openSortDialog() {
	a.openSortDialogForPanel(a.model.ActivePanel)
}

func (a *App) openSortDialogForPanel(panelID int) {
	a.closeListingFormatDialog()
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
	st := &a.model.SortDialog
	a.handleLinearFormDialogKey(event, form, linearFormHandlers{
		focus:              &st.Focus,
		onApply:            a.applySortDialog,
		onCancel:           a.closeSortDialog,
		allowPlainOKCancel: true,
		onMnemonic: func(r rune) bool {
			switch r {
			case 'n', 'N':
				st.SortMode = panel.SortName
				st.Focus = 0
			case 'e', 'E':
				st.SortMode = panel.SortExtension
				st.Focus = 1
			case 's', 'S':
				st.SortMode = panel.SortSize
				st.Focus = 2
			case 'm', 'M':
				st.SortMode = panel.SortMtime
				st.Focus = 3
			case 'u', 'U':
				st.DiskUsageIdleSizeSort = !st.DiskUsageIdleSizeSort
				st.Focus = 4
			case 'r', 'R':
				st.SortReverse = !st.SortReverse
				st.Focus = 5
			case 'd', 'D':
				st.DirectoriesFirst = !st.DirectoriesFirst
				st.Focus = 6
			default:
				return false
			}
			return true
		},
		onSpace: func(focus int) bool {
			switch focus {
			case 0, 1, 2, 3:
				modes := []panel.SortMode{panel.SortName, panel.SortExtension, panel.SortSize, panel.SortMtime}
				st.SortMode = modes[focus]
			case 4:
				st.DiskUsageIdleSizeSort = !st.DiskUsageIdleSizeSort
			case 5:
				st.SortReverse = !st.SortReverse
			case 6:
				st.DirectoriesFirst = !st.DirectoriesFirst
			case form.OKIndex():
				a.applySortDialog()
			case form.CancelIndex():
				a.closeSortDialog()
			default:
				return false
			}
			return true
		},
	})
}

func listingFormatFromShortcut(ch rune, focus *int) (panel.ListFormat, bool) {
	for i, row := range panel.ListFormatDialogRadios() {
		if unicode.ToLower(ch) == unicode.ToLower(row.Shortcut) {
			*focus = i
			return row.Format, true
		}
	}
	return 0, false
}

func scrollModeFromShortcut(ch rune, focus *int) (panel.ScrollMode, bool) {
	for i, row := range panel.ScrollModeDialogRadios() {
		if unicode.ToLower(ch) == unicode.ToLower(row.Shortcut) {
			*focus = 3 + i
			return row.Mode, true
		}
	}
	return "", false
}

func (a *App) handleListingFormatDialogKey(event *tcell.EventKey) {
	form := ui.NewDialogLinearForm(3)
	st := &a.model.ListingFormatDialog
	radios := panel.ListFormatDialogRadios()
	a.handleLinearFormDialogKey(event, form, linearFormHandlers{
		focus:              &st.Focus,
		onApply:            a.applyListingFormatDialog,
		onCancel:           a.closeListingFormatDialog,
		allowPlainOKCancel: true,
		onMnemonic: func(r rune) bool {
			if format, ok := listingFormatFromShortcut(r, &st.Focus); ok {
				st.ListFormat = format
				return true
			}
			return false
		},
		onSpace: func(focus int) bool {
			switch focus {
			case 0, 1, 2:
				st.ListFormat = radios[focus].Format
			case form.OKIndex():
				a.applyListingFormatDialog()
			case form.CancelIndex():
				a.closeListingFormatDialog()
			default:
				return false
			}
			return true
		},
	})
}

func (a *App) openListingFormatDialog() {
	a.openListingFormatDialogForPanel(a.model.ActivePanel)
}

func (a *App) openListingFormatDialogForPanel(panelID int) {
	a.closeSortDialog()
	a.clearTransientMessage()
	target := a.panelByID(panelID)
	a.model.ListingFormatDialog = ui.ListingFormatDialogState{
		Open:       true,
		ListFormat: panel.EffectiveListFormat(target.ListFormat),
		Focus:      0,
		PanelID:    panelID,
	}
}

func (a *App) closeListingFormatDialog() {
	a.model.ListingFormatDialog.Open = false
}

func (a *App) applyListingFormatDialog() {
	st := a.model.ListingFormatDialog
	target := a.panelByID(st.PanelID)
	target.ListFormat = panel.EffectiveListFormat(st.ListFormat)
	a.setTransientMessage(fmt.Sprintf("%s listing: %s", panelLabel(st.PanelID), target.ListFormat.String()), ui.MessageUrgencyInfo)
	a.closeListingFormatDialog()
}

func (a *App) openConfigDialog() {
	a.clearTransientMessage()
	lf, _ := panel.ParseListFormat(a.config.DefaultListingFormat)
	sm, _ := panel.ParseScrollMode(a.config.UI.ScrollMode)
	a.model.ConfigDialog = ui.ConfigDialogState{
		Open:                  true,
		ShowFileIcons:         a.config.UI.ShowFileIcons,
		ZoomActivePanel:       a.config.UI.ZoomActivePanel,
		ShrunkenShowsNameOnly: a.config.UI.ShrunkenShowsNameOnly,
		ScrollMode:            panel.EffectiveScrollMode(sm),
		ListFormat:            panel.EffectiveListFormat(lf),
		Focus:                 0,
	}
}

func (a *App) closeConfigDialog() {
	a.model.ConfigDialog.Open = false
}

func (a *App) applyConfigDialog() {
	a.zoomActivePanelOverride = nil
	val := a.model.ConfigDialog.ShowFileIcons
	zoom := a.model.ConfigDialog.ZoomActivePanel
	shrunken := a.model.ConfigDialog.ShrunkenShowsNameOnly
	scrollMode := panel.ScrollModeTOMLValue(a.model.ConfigDialog.ScrollMode)
	lf := panel.EffectiveListFormat(a.model.ConfigDialog.ListFormat)
	a.config.UI.ShowFileIcons = val
	a.config.UI.ZoomActivePanel = zoom
	a.config.UI.ShrunkenShowsNameOnly = shrunken
	a.config.UI.ScrollMode = scrollMode
	a.config.DefaultListingFormat = panel.ListingFormatTOMLValue(lf)
	a.model.ShowFileIcons = val
	a.model.ShrunkenShowsNameOnly = shrunken
	a.model.Left.ListFormat = lf
	a.model.Right.ListFormat = lf
	a.syncScrollFromConfig()
	a.closeConfigDialog()
	msg := "Configuration saved"
	patch := map[string]interface{}{
		"ui": map[string]interface{}{
			"show_file_icons":          val,
			"zoom_active_panel":        zoom,
			"shrunken_shows_name_only": shrunken,
			"scroll_mode":              scrollMode,
		},
		"default_listing_format": panel.ListingFormatTOMLValue(lf),
	}
	if err := a.persistPartial(patch); err != nil {
		msg = fmt.Sprintf("Configuration saved (could not write config: %v)", err)
	}
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
	a.ensurePanelsVisible()
}

func (a *App) handleConfigDialogKey(event *tcell.EventKey) {
	form := ui.NewDialogLinearForm(9)
	st := &a.model.ConfigDialog
	listRadios := panel.ListFormatDialogRadios()
	scrollRadios := panel.ScrollModeDialogRadios()
	a.handleLinearFormDialogKey(event, form, linearFormHandlers{
		focus:              &st.Focus,
		onApply:            a.applyConfigDialog,
		onCancel:           a.closeConfigDialog,
		allowPlainOKCancel: true,
		onMnemonic: func(r rune) bool {
			if mode, ok := scrollModeFromShortcut(r, &st.Focus); ok {
				st.ScrollMode = mode
				return true
			}
			for i, row := range listRadios {
				if unicode.ToLower(r) == unicode.ToLower(row.Shortcut) {
					st.ListFormat = row.Format
					st.Focus = 6 + i
					return true
				}
			}
			switch r {
			case 'f', 'F':
				st.ShowFileIcons = !st.ShowFileIcons
				st.Focus = 0
			case 'z', 'Z':
				st.ZoomActivePanel = !st.ZoomActivePanel
				st.Focus = 1
			case 's', 'S':
				st.ShrunkenShowsNameOnly = !st.ShrunkenShowsNameOnly
				st.Focus = 2
			default:
				return false
			}
			return true
		},
		onSpace: func(focus int) bool {
			switch focus {
			case 0:
				st.ShowFileIcons = !st.ShowFileIcons
			case 1:
				st.ZoomActivePanel = !st.ZoomActivePanel
			case 2:
				st.ShrunkenShowsNameOnly = !st.ShrunkenShowsNameOnly
			case 3, 4, 5:
				st.ScrollMode = scrollRadios[focus-3].Mode
			case 6, 7, 8:
				st.ListFormat = listRadios[focus-6].Format
			case form.OKIndex():
				a.applyConfigDialog()
			case form.CancelIndex():
				a.closeConfigDialog()
			default:
				return false
			}
			return true
		},
	})
}

// Group selection dialog handlers

func (a *App) openGroupSelect(mode string, context string) {
	if context == "" {
		context = "panel"
	}
	a.model.GroupSelect = ui.GroupSelectState{
		Open:             true,
		Text:             "",
		Mode:             mode,
		Context:          context,
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
	context := gs.Context
	if context == "" {
		context = "panel"
	}
	switch context {
	case "find":
		a.findCtrl.ApplyGroupSelect(gs.Mode, gs.Text, gs.FilesOnly, gs.CaseSensitive, gs.UseShellPatterns)
	default:
		p := a.activePanel()
		if gs.Mode == "select" {
			p.SelectGroup(gs.Text, gs.FilesOnly, gs.CaseSensitive, gs.UseShellPatterns)
			a.setTransientMessage(fmt.Sprintf("Selected matching %q", gs.Text), ui.MessageUrgencyInfo)
		} else {
			p.UnselectGroup(gs.Text, gs.FilesOnly, gs.CaseSensitive, gs.UseShellPatterns)
			a.setTransientMessage(fmt.Sprintf("Unselected matching %q", gs.Text), ui.MessageUrgencyInfo)
		}
	}
	a.closeGroupSelect()
	if context == "find" && a.model.FindDialog.Open {
		a.paintFindDialogOverlay()
	}
}

// confirmGroupSelectFromInput applies the pattern row then runs OK (Enter / Alt+O).
func (a *App) confirmGroupSelectFromInput() {
	gs := &a.model.GroupSelect
	if gs.Focus == 0 {
		e := groupSelectScrollingQuery(gs, a.groupSelectQueryWidth())
		e.apply()
	}
	a.executeGroupSelect()
}

func (a *App) handleGroupSelectKey(event *tcell.EventKey) {
	gs := &a.model.GroupSelect
	form := ui.NewDialogLinearForm(4)

	if ui.AltDialogOK(event) {
		a.confirmGroupSelectFromInput()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeGroupSelect()
		return
	}

	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyF9:
		a.closeGroupSelect()
		return
	case tcell.KeyEnter:
		if gs.Focus == form.CancelIndex() {
			a.closeGroupSelect()
		} else {
			a.confirmGroupSelectFromInput()
		}
		return
	}

	if gs.Focus == 0 {
		skipScrolling := event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) && groupSelectAltIsDialogMnemonic(event.Rune())
		if !skipScrolling && a.handleScrollingQueryKey(event, true, groupSelectScrollingQuery(gs, a.groupSelectQueryWidth())) {
			return
		}
	}

	switch event.Key() {
	case tcell.KeyRune:
		// Mnemonics follow dialog standards: Alt+letter only (plain typing goes into the pattern).
		if keymap.AltLetterModifiers(event.Modifiers()) {
			switch event.Rune() {
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
			switch gs.Focus {
			case 1:
				gs.FilesOnly = !gs.FilesOnly
			case 2:
				gs.CaseSensitive = !gs.CaseSensitive
			case 3:
				gs.UseShellPatterns = !gs.UseShellPatterns
			case form.OKIndex():
				a.confirmGroupSelectFromInput()
			case form.CancelIndex():
				a.closeGroupSelect()
			}
			break
		}
	}
	if focus, ok := form.MoveFocus(gs.Focus, event.Key()); ok {
		gs.Focus = focus
	}
}

func groupSelectAltIsDialogMnemonic(r rune) bool {
	switch r {
	case 'f', 'F', 's', 'S', 'u', 'U':
		return true
	default:
		return false
	}
}
