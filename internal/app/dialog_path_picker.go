package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/pathpick"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) closePathPicker() {
	purpose := a.model.PathPicker.Purpose
	a.stopPathPickerValidateTimer()
	a.pathPickerValidateGen.Add(1)
	a.model.PathPicker = ui.PathPickerState{}
	if a.model.TransferDialog.Open && a.model.TransferDialog.Phase == ui.TransferPhaseDestination &&
		purpose == ui.PathPickerPurposeApplyTransferDestination {
		a.armTransferDestinationValidateTimer()
	}
}

func (a *App) syncPathPickerRanks() {
	st := &a.model.PathPicker
	if !st.Open {
		return
	}
	lines := make([]string, len(st.Items))
	for i, e := range st.Items {
		lines[i] = e.SearchLine()
	}
	q := search.Parse(st.Query)
	opts := search.Options{CaseInsensitive: a.config.CaseInsensitiveFilter}
	ranked := q.Rank(lines, opts)
	st.Ranked = make([]int, len(ranked))
	st.MatchRanges = make([][]search.Range, len(st.Items))
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
	ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
}

func (a *App) syncPathPickerCompletion() {
	st := &a.model.PathPicker
	if !st.Open {
		return
	}
	panel := a.activePanel()
	c, ok := pathpick.SuggestAtCursor(panel.PathString(), a.model.UserHomeDir, st.Query, st.QueryCursor, a.config.ShowHidden)
	if !ok {
		st.QueryCompletionSuffix = ""
		st.QueryCompletionIsDir = false
		a.syncPathPickerScroll()
		return
	}
	st.QueryCompletionSuffix = c.Suffix
	st.QueryCompletionIsDir = c.IsDir
	a.syncPathPickerScroll()
}

func (a *App) syncPathPickerScroll() {
	st := &a.model.PathPicker
	if !st.Open {
		return
	}
	width := a.pathPickerQueryWidth()
	valueLen := len([]rune(st.Query))
	suffixLen := len([]rune(st.QueryCompletionSuffix))
	st.QueryCursor, st.QueryScroll = ui.EnsurePathInputScroll(valueLen, st.QueryCursor, st.QueryScroll, width, suffixLen)
}

func (a *App) pathPickerScrollToCaret() {
	st := &a.model.PathPicker
	if !st.Open {
		return
	}
	width := a.pathPickerQueryWidth()
	valueLen := len([]rune(st.Query))
	suffixLen := len([]rune(st.QueryCompletionSuffix))
	st.QueryCursor, st.QueryScroll = ui.EnsurePathInputScroll(valueLen, st.QueryCursor, st.QueryScroll, width, suffixLen)
}

func (a *App) acceptPathPickerCompletion() {
	st := &a.model.PathPicker
	if st.QueryCompletionSuffix == "" {
		return
	}
	runes := []rune(st.Query)
	pos := st.QueryCursor
	if pos < 0 {
		pos = 0
	}
	if pos > len(runes) {
		pos = len(runes)
	}
	suffix := []rune(st.QueryCompletionSuffix)
	newRunes := make([]rune, 0, len(runes)+len(suffix)+1)
	newRunes = append(newRunes, runes[:pos]...)
	newRunes = append(newRunes, suffix...)
	newRunes = append(newRunes, runes[pos:]...)
	st.Query = string(newRunes)
	if st.QueryCompletionIsDir {
		st.Query += "/"
	}
	st.QueryCursor = len([]rune(st.Query))
	st.QueryCompletionSuffix = ""
	st.QueryCompletionIsDir = false
	a.pathPickerScrollToCaret()

	a.syncPathPickerRanks()
	a.syncPathPickerCompletion()
	a.armPathPickerValidateTimer()
	st.Selected = 0
	ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
}

func (a *App) pathPickerListRows() int {
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

func (a *App) activatePathPickerSelection() {
	st := &a.model.PathPicker
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Items) {
		return
	}
	path := filepath.Clean(st.Items[entIdx].Path)

	switch st.Purpose {
	case ui.PathPickerPurposeNavigate:
		p := a.activePanel()
		if err := a.navigatePanelToDirectory(a.model.ActivePanel, path, ""); err != nil {
			a.setErrorMessage("Bookmark", err)
			return
		}
		p.EnsureCursorVisible(a.activeViewportRows())
		a.closePathPicker()
		a.setTransientMessage(path, ui.MessageUrgencyInfo)
	case ui.PathPickerPurposeApplyTransferDestination:
		d := &a.model.TransferDialog
		rn := []rune(path)
		d.Destination.Value = path
		d.Destination.Cursor = len(rn)
		d.Destination.Prefill = ""
		d.Destination.PrefillPending = false
		d.DestSubFocus = ui.TransferDestSubFocusText
		a.closePathPicker()
	case ui.PathPickerPurposeApplyFileDialogField:
		idx := st.FileFieldIndex
		if idx < 0 || idx >= len(a.model.FileDialog.Fields) {
			a.closePathPicker()
			return
		}
		f := &a.model.FileDialog.Fields[idx]
		f.Value = path
		f.Cursor = len([]rune(path))
		f.Prefill = ""
		f.PrefillPending = false
		f.PickerFocused = false
		a.closePathPicker()
	default:
		a.closePathPicker()
	}
}

func (a *App) handlePathPickerKey(event *tcell.EventKey) {
	st := &a.model.PathPicker
	if ui.AltDialogOK(event) {
		a.activatePathPickerSelection()
		return
	}
	if ui.AltDialogCancel(event) {
		a.closePathPicker()
		return
	}

	if st.Focus == 0 && a.handleScrollingQueryKey(event, true, a.pathPickerScrollingQuery()) {
		return
	}

	switch event.Key() {
	case tcell.KeyEsc:
		a.closePathPicker()
	case tcell.KeyEnter:
		switch st.Focus {
		case 2:
			a.closePathPicker()
		default:
			a.activatePathPickerSelection()
		}
	case tcell.KeyTab:
		if st.Focus == 0 && st.QueryCompletionSuffix != "" {
			a.acceptPathPickerCompletion()
			return
		}
		st.Focus = (st.Focus + 1) % 3
	case tcell.KeyBacktab:
		st.Focus = (st.Focus + 2) % 3
	case tcell.KeyUp:
		switch st.Focus {
		case 0:
			if len(st.Ranked) > 0 {
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -1)
				ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
			}
		default:
			st.Focus = 0
			ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		}
	case tcell.KeyDown:
		switch st.Focus {
		case 0:
			if len(st.Ranked) > 0 {
				st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), 1)
				ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
			}
		case 1:
			st.Focus = 2
		}
	case tcell.KeyHome:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = 0
			ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		}
	case tcell.KeyEnd:
		if st.Focus == 0 && event.Modifiers()&tcell.ModCtrl != 0 && len(st.Ranked) > 0 {
			st.Selected = len(st.Ranked) - 1
			ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		}
	case tcell.KeyPgUp:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.pathPickerListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), -step)
			ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		}
	case tcell.KeyPgDn:
		if st.Focus == 0 && len(st.Ranked) > 0 {
			step := max(1, a.pathPickerListRows()-1)
			st.Selected = ui.ListClampedSelectionDelta(st.Selected, len(st.Ranked), step)
			ui.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		}
	case tcell.KeyLeft:
		switch st.Focus {
		case 1:
			st.Focus = 0
		case 2:
			st.Focus = 1
		}
	case tcell.KeyRight:
		if st.Focus == 1 {
			st.Focus = 2
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		if st.Focus == 0 {
			break
		}
		switch event.Rune() {
		case 'o', 'O':
			a.activatePathPickerSelection()
		case 'c', 'C':
			a.closePathPicker()
		case ' ':
			switch st.Focus {
			case 1:
				a.activatePathPickerSelection()
			case 2:
				a.closePathPicker()
			}
		}
	}
}

// pathPickerQueryWidth returns the visible width of the query input row.
// Mirrors the layout in drawPathPickerDialog: rect.Width - 4 with rect width = 78
// clamped to layout.Width - 4.
func (a *App) pathPickerQueryWidth() int {
	termW, _ := a.screen.Size()
	width := 78
	if width > termW-4 {
		width = termW - 4
	}
	if width < 36 {
		width = 36
	}
	return width - 4
}

// pathPickerItemsHistoryAndBookmarks returns merged passive-first panel histories plus
// bookmarks (deduped by cleaned path), each with a display line for fuzzy matching.
func (a *App) pathPickerItemsHistoryAndBookmarks() ([]ui.PathPickerItem, error) {
	passive := a.inactivePanel()
	active := a.activePanel()
	seen := make(map[string]struct{})
	var items []ui.PathPickerItem

	panelPath := active.PathString()
	home := a.model.UserHomeDir
	addPath := func(p string) {
		cp := filepath.Clean(p)
		if cp == "." || cp == "" {
			return
		}
		if pathpick.QueryLooksPathlike(p) && pathEntryMissing(panelPath, home, cp) {
			return
		}
		key := cp
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		items = append(items, ui.PathPickerItem{
			Source: "history",
			Path:   cp,
		})
	}

	for _, p := range passive.History {
		addPath(p)
	}
	for _, p := range active.History {
		addPath(p)
	}

	marks, err := bookmarks.LoadAll(a.config.Bookmarks.File, a.model.UserHomeDir)
	if err != nil {
		return items, err
	}
	for i := range marks {
		cp := filepath.Clean(marks[i].Path)
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		items = append(items, ui.PathPickerItem{
			Source: marks[i].Origin.PathPickerSource(),
			Name:   marks[i].Name,
			Path:   cp,
		})
	}
	return items, nil
}

func pathEntryMissing(panelPath, home, path string) bool {
	if strings.HasPrefix(path, "sftp://") {
		return pathpick.TypedDoesNotExist(panelPath, home, path)
	}
	_, err := os.Lstat(path)
	return err != nil
}

func (a *App) openPathPickerForTransfer() {
	a.stopTransferDestinationValidateTimer()
	a.transferDestValidateGen.Add(1)
	items, err := a.pathPickerItemsHistoryAndBookmarks()
	if err != nil {
		a.setErrorMessage("Choose path", err)
		return
	}
	if len(items) == 0 {
		a.setTransientMessage("No paths in history or bookmarks", ui.MessageUrgencyInfo)
		return
	}
	a.model.PathPicker = ui.PathPickerState{
		Open:       true,
		Title:      "Choose path",
		Purpose:    ui.PathPickerPurposeApplyTransferDestination,
		Query:      "",
		Items:      items,
		Focus:      0,
		Selected:   0,
		ListScroll: 0,
	}
	a.syncPathPickerRanks()
}

func (a *App) openPathPickerForFileField(fieldIndex int) {
	items, err := a.pathPickerItemsHistoryAndBookmarks()
	if err != nil {
		a.setErrorMessage("Choose path", err)
		return
	}
	if len(items) == 0 {
		a.setTransientMessage("No paths in history or bookmarks", ui.MessageUrgencyInfo)
		return
	}
	a.model.PathPicker = ui.PathPickerState{
		Open:           true,
		Title:          "Choose path",
		Purpose:        ui.PathPickerPurposeApplyFileDialogField,
		FileFieldIndex: fieldIndex,
		Query:          "",
		Items:          items,
		Focus:          0,
		Selected:       0,
		ListScroll:     0,
	}
	a.syncPathPickerRanks()
}

func (a *App) stopPathPickerValidateTimer() {
	if a.pathPickerValidateTimer == nil {
		return
	}
	if !a.pathPickerValidateTimer.Stop() {
		select {
		case <-a.pathPickerValidateTimer.C:
		default:
		}
	}
	a.pathPickerValidateTimer = nil
}

func (a *App) armPathPickerValidateTimer() {
	if !a.model.PathPicker.Open {
		return
	}
	a.stopPathPickerValidateTimer()
	st := &a.model.PathPicker
	st.QueryPathCheckPending = true
	gen := a.pathPickerValidateGen.Add(1)
	delay := time.Duration(a.config.UI.PathPickerValidateDelayMS) * time.Millisecond
	a.pathPickerValidateTimer = time.AfterFunc(delay, func() {
		a.pathPickerValidateTimer = nil
		if a.pathPickerValidateGen.Load() != gen {
			return
		}
		if !a.model.PathPicker.Open {
			return
		}
		a.applyPathPickerPathValidation()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(pathPickerValidatePayload{}))
	})
}

func (a *App) applyPathPickerPathValidation() {
	st := &a.model.PathPicker
	if !st.Open {
		return
	}
	st.QueryPathCheckPending = false
	panel := a.activePanel()
	st.QueryPathInvalid = pathpick.TypedDoesNotExist(panel.PathString(), a.model.UserHomeDir, st.Query)
	a.syncOpenPathInputsAfterFSChange()
}

func (a *App) stopTransferDestinationValidateTimer() {
	if a.transferDestValidateTimer == nil {
		return
	}
	if !a.transferDestValidateTimer.Stop() {
		select {
		case <-a.transferDestValidateTimer.C:
		default:
		}
	}
	a.transferDestValidateTimer = nil
}

func (a *App) armTransferDestinationValidateTimer() {
	if !a.model.TransferDialog.Open || a.model.TransferDialog.Phase != ui.TransferPhaseDestination {
		return
	}
	a.stopTransferDestinationValidateTimer()
	d := &a.model.TransferDialog
	d.DestPathCheckPending = true
	gen := a.transferDestValidateGen.Add(1)
	delay := time.Duration(a.config.UI.PathPickerValidateDelayMS) * time.Millisecond
	a.transferDestValidateTimer = time.AfterFunc(delay, func() {
		a.transferDestValidateTimer = nil
		if a.transferDestValidateGen.Load() != gen {
			return
		}
		if !a.model.TransferDialog.Open || a.model.TransferDialog.Phase != ui.TransferPhaseDestination {
			return
		}
		a.applyTransferDestinationPathValidation()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(transferDestValidatePayload{}))
	})
}

func (a *App) applyTransferDestinationPathValidation() {
	d := &a.model.TransferDialog
	if !d.Open || d.Phase != ui.TransferPhaseDestination {
		return
	}
	d.DestPathCheckPending = false
	panel := a.activePanel()
	d.DestPathInvalid = pathpick.TypedDoesNotExist(panel.PathString(), a.model.UserHomeDir, d.Destination.Value)
	a.syncOpenPathInputsAfterFSChange()
}
