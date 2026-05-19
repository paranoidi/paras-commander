package app

import (
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/search"
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
	q := search.Parse(st.Query)
	opts := search.Options{CaseInsensitive: a.config.CaseInsensitiveFilter}
	ranked := q.Rank(lines, opts)
	st.Ranked = make([]int, len(ranked))
	st.MatchRanges = make([][]search.Range, len(st.Paths))
	for i := range st.MatchRanges {
		st.MatchRanges[i] = nil
	}
	for i, r := range ranked {
		st.Ranked[i] = r.Index
		if r.Index >= 0 && r.Index < len(st.MatchRanges) {
			st.MatchRanges[r.Index] = r.Result.Ranges
		}
	}
	if st.Selected >= len(st.Ranked) {
		if len(st.Ranked) == 0 {
			st.Selected = 0
		} else {
			st.Selected = len(st.Ranked) - 1
		}
	}
	if st.Selected < 0 {
		st.Selected = 0
	}
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
	if ui.AltDialogOK(event) {
		a.activateHistorySelection()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closeHistoryDialog()
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
		st := &a.model.HistoryDialog
		if nf, ok := ui.ListOKCancelNavFocusKey(st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
			}
			break
		}
		if st.Focus == 0 && len(st.Ranked) > 0 {
			switch event.Key() {
			case tcell.KeyUp:
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
				ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
			case tcell.KeyDown:
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
				ui.EnsureHistoryListScroll(st, a.historyDialogListRows())
			}
		}
	case tcell.KeyHome:
		if a.model.HistoryDialog.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(a.model.HistoryDialog.Ranked) > 0 {
			a.model.HistoryDialog.Selected = 0
			ui.EnsureHistoryListScroll(&a.model.HistoryDialog, a.historyDialogListRows())
		}
	case tcell.KeyEnd:
		if a.model.HistoryDialog.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(a.model.HistoryDialog.Ranked) > 0 {
			a.model.HistoryDialog.Selected = len(a.model.HistoryDialog.Ranked) - 1
			ui.EnsureHistoryListScroll(&a.model.HistoryDialog, a.historyDialogListRows())
		}
	case tcell.KeyPgUp:
		if a.model.HistoryDialog.Focus == 0 && len(a.model.HistoryDialog.Ranked) > 0 {
			step := max(1, a.historyDialogListRows()-1)
			a.model.HistoryDialog.Selected = ui.ListClampedSelectionDelta(
				a.model.HistoryDialog.Selected, len(a.model.HistoryDialog.Ranked), -step)
			ui.EnsureHistoryListScroll(&a.model.HistoryDialog, a.historyDialogListRows())
		}
	case tcell.KeyPgDn:
		if a.model.HistoryDialog.Focus == 0 && len(a.model.HistoryDialog.Ranked) > 0 {
			step := max(1, a.historyDialogListRows()-1)
			a.model.HistoryDialog.Selected = ui.ListClampedSelectionDelta(
				a.model.HistoryDialog.Selected, len(a.model.HistoryDialog.Ranked), step)
			ui.EnsureHistoryListScroll(&a.model.HistoryDialog, a.historyDialogListRows())
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
