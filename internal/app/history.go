package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openHistoryDialog(panelID int) {
	if ui.IsAuxiliaryView(a.model.ViewMode) {
		return
	}
	if a.inQuickFilterUI() {
		a.activePanel().CancelFilter(a.activeViewportRows())
	}
	p := a.panelByID(panelID)
	paths := append([]string(nil), p.History...)
	if len(paths) == 0 {
		a.setTransientMessage("No directory history yet", ui.MessageUrgencyInfo)
		return
	}
	curIdx := p.HistoryIndex
	if curIdx < 0 || curIdx >= len(paths) {
		curIdx = 0
	}
	display := historyDisplayLines(paths, curIdx)
	a.model.HistoryDialog = ui.HistoryDialogState{
		Open:         true,
		PanelID:      panelID,
		Paths:        paths,
		CurrentIndex: curIdx,
		DisplayLines: display,
		Query:        "",
		Focus:        0,
		Selected:     0,
		ListScroll:   0,
	}
	a.syncHistoryDialogRanks()
	for i, idx := range a.model.HistoryDialog.Ranked {
		if idx == curIdx {
			a.model.HistoryDialog.Selected = i
			break
		}
	}
	ui.EnsureHistoryListScroll(&a.model.HistoryDialog, a.historyDialogListRows())
}

func historyDisplayLines(paths []string, currentIndex int) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		prefix := "  "
		if i == currentIndex {
			prefix = "* "
		}
		out[i] = prefix + p
	}
	return out
}

func (a *App) closeHistoryDialog() {
	a.model.HistoryDialog = ui.HistoryDialogState{}
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
