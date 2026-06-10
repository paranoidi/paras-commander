package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func (a *App) mergedPanelHistories() []string {
	return panel.MergeNavigationHistories(a.inactivePanel().History, a.activePanel().History)
}

func historyMarkPath(st *ui.HistoryDialogState) string {
	idx := st.PanelCurrentIndex
	if idx < 0 || idx >= len(st.PanelPaths) {
		return ""
	}
	return panel.CleanPathString(st.PanelPaths[idx])
}

func historyDisplayLinesFor(paths []string, markPath string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		prefix := "  "
		if markPath != "" && panel.CleanPathString(p) == markPath {
			prefix = "* "
		}
		out[i] = prefix + p
	}
	return out
}

func (a *App) openHistoryDialog(panelID int) {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
	p := a.panelByID(panelID)
	panelPaths := append([]string(nil), p.History...)
	if len(panelPaths) == 0 {
		a.setTransientMessage("No directory history yet", ui.MessageUrgencyInfo)
		return
	}
	curIdx := p.HistoryIndex
	if curIdx < 0 || curIdx >= len(panelPaths) {
		curIdx = 0
	}
	display := historyDisplayLinesFor(panelPaths, historyMarkPath(&ui.HistoryDialogState{
		PanelPaths:        panelPaths,
		PanelCurrentIndex: curIdx,
	}))
	a.model.HistoryDialog = ui.HistoryDialogState{
		Open:              true,
		PanelID:           panelID,
		Paths:             panelPaths,
		CurrentIndex:      curIdx,
		BothPanels:        false,
		PanelPaths:        panelPaths,
		PanelCurrentIndex: curIdx,
		DisplayLines:      display,
		Query:             "",
		Focus:             0,
		Selected:          0,
		ListScroll:        0,
	}
	a.syncHistoryDialogRanks()
	selected := 0
	for i, idx := range a.model.HistoryDialog.Ranked {
		if idx == curIdx {
			selected = i
			break
		}
	}
	a.model.HistoryDialog.Selected = selected
	ui.EnsureHistoryListScroll(&a.model.HistoryDialog, a.historyDialogListRows())
}

func (a *App) closeHistoryDialog() {
	a.model.HistoryDialog = ui.HistoryDialogState{}
}

func (a *App) toggleHistoryDialogBothPanels() {
	st := &a.model.HistoryDialog
	if !st.Open {
		return
	}
	if st.BothPanels {
		st.BothPanels = false
		st.Paths = append([]string(nil), st.PanelPaths...)
	} else {
		st.BothPanels = true
		st.Paths = a.mergedPanelHistories()
	}
	a.reloadHistoryDialogList()
}

func (a *App) reloadHistoryDialogList() {
	st := &a.model.HistoryDialog
	if !st.Open {
		return
	}
	var selectedPath string
	if len(st.Ranked) > 0 && st.Selected >= 0 && st.Selected < len(st.Ranked) {
		idx := st.Ranked[st.Selected]
		if idx >= 0 && idx < len(st.Paths) {
			selectedPath = st.Paths[idx]
		}
	}
	st.DisplayLines = historyDisplayLinesFor(st.Paths, historyMarkPath(st))
	a.syncHistoryDialogRanks()
	if selectedPath != "" {
		want := panel.CleanPathString(selectedPath)
		for i, idx := range st.Ranked {
			if idx >= 0 && idx < len(st.Paths) && panel.CleanPathString(st.Paths[idx]) == want {
				st.Selected = i
				break
			}
		}
	}
	ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
}

func historyDialogOverlayFooterKeys(keys *keymap.Map, bothPanels bool) []menu.FunctionKey {
	if keys == nil {
		return nil
	}
	hint := "Both Panels"
	if bothPanels {
		hint = "Active panel"
	}
	if lbl := keys.MenuBindingLabel(keymap.ActionPanelHistoryBothPanels); lbl != "" {
		return []menu.FunctionKey{{KeyLabel: lbl, Hint: hint}}
	}
	return nil
}

func (a *App) syncHistoryDialogRanks() {
	st := &a.model.HistoryDialog
	if !st.Open {
		return
	}
	lines := make([]string, len(st.DisplayLines))
	copy(lines, st.DisplayLines)
	st.Ranked, st.MatchRanges = syncFilteredListRanks(lines, st.Query, len(st.Paths), a.config.CaseInsensitiveFilter)
	clampFilteredListSelection(&st.Selected, len(st.Ranked))
	ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
}

func (a *App) historyDialogListRows() int {
	termW, termH := a.screen.Size()
	layout := a.layoutForTerminalSize(termW, termH)
	listH := layout.Height - 12
	switch {
	case listH > 18:
		listH = 18
	case listH < 4:
		listH = 4
	}
	dialogHeight := 9 + listH
	if dialogHeight > layout.Height-2 {
		listH = layout.Height - 2 - 9
		if listH < 4 {
			return 4
		}
	}
	return listH
}

func (a *App) activateHistorySelection() {
	st := &a.model.HistoryDialog
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Paths) {
		return
	}
	path := filepath.Clean(st.Paths[entIdx])
	if err := a.navigatePanelToDirectory(st.PanelID, path, ""); err != nil {
		a.setErrorMessage("History", err)
		return
	}
	a.model.ActivePanel = st.PanelID
	a.model.ActiveSubFocus = ui.SubFocusFileList
	a.closeHistoryDialog()
	a.setTransientMessage(path, ui.MessageUrgencyInfo)
}

func (a *App) handleHistoryDialogKey(event *tcell.EventKey) {
	if a.keysHistoryDialog != nil {
		if id, ok := a.keysHistoryDialog.Lookup(event); ok {
			switch id {
			case keymap.ActionPanelHistoryBothPanels:
				a.toggleHistoryDialogBothPanels()
				return
			}
		}
	}
	if a.tryStandardDialogActions(event, a.activateHistorySelection, a.closeHistoryDialog, nil) {
		return
	}

	st := &a.model.HistoryDialog
	if st.Focus == 0 {
		onChange := func() {
			a.syncHistoryDialogRanks()
			st.Selected = 0
			ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
		}
		if a.handleScrollingQueryKey(event, true, historyDialogScrollingQuery(st, a.historyDialogQueryWidth(), onChange)) {
			return
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closeHistoryDialog()
	case tcell.KeyEnter:
		switch a.model.HistoryDialog.Focus {
		case 2:
			a.closeHistoryDialog()
		default:
			a.activateHistorySelection()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := ui.ListOKCancelNavFocusKey(st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
			}
			break
		}
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.historyDialogListRows, func() {
			ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
		}) {
			break
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.historyDialogListRows, func() {
			ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
		}) {
			break
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		if a.model.HistoryDialog.Focus == 0 {
			break
		}
		switch event.Rune() {
		case 'o', 'O':
			a.activateHistorySelection()
		case 'c', 'C':
			a.closeHistoryDialog()
		case ' ':
			switch a.model.HistoryDialog.Focus {
			case 1:
				a.activateHistorySelection()
			case 2:
				a.closeHistoryDialog()
			}
		}
	}
}
