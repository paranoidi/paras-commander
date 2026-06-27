package app

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/app/pathpick"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) closePathPicker() {
	purpose := a.model.PathPicker.Purpose
	a.pathPickerValidate.Invalidate()
	a.model.PathPicker = dialog.PathPickerState{}
	if a.model.TransferDialog.Open && a.model.TransferDialog.Phase == dialog.TransferPhaseDestination &&
		purpose == dialog.PathPickerPurposeApplyTransferDestination {
		a.armTransferDestinationValidateTimer()
	}
	if a.model.FlattenDialog.Open && purpose == dialog.PathPickerPurposeApplyFlattenDestination {
		a.armFlattenDestinationValidateTimer()
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
	st.Ranked, st.MatchRanges = syncFilteredListRanks(lines, st.Query, len(st.Items), a.config.CaseInsensitiveFilter)
	clampFilteredListSelection(&st.Selected, len(st.Ranked))
	dialog.EnsurePathPickerListScroll(st, a.pathPickerListRows())
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
	st.QueryCursor, st.QueryScroll = dialog.EnsurePathInputScroll(valueLen, st.QueryCursor, st.QueryScroll, width, suffixLen)
}

func (a *App) pathPickerScrollToCaret() {
	st := &a.model.PathPicker
	if !st.Open {
		return
	}
	width := a.pathPickerQueryWidth()
	valueLen := len([]rune(st.Query))
	suffixLen := len([]rune(st.QueryCompletionSuffix))
	st.QueryCursor, st.QueryScroll = dialog.EnsurePathInputScroll(valueLen, st.QueryCursor, st.QueryScroll, width, suffixLen)
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
	dialog.EnsurePathPickerListScroll(st, a.pathPickerListRows())
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
	case dialog.PathPickerPurposeNavigate:
		p := a.activePanel()
		if err := a.navigatePanelToDirectory(a.model.ActivePanel, path, ""); err != nil {
			a.setErrorMessage("Bookmark", err)
			return
		}
		p.EnsureCursorVisible(a.activeViewportRows())
		a.closePathPicker()
		a.setTransientMessage(path, ui.MessageUrgencyInfo)
	case dialog.PathPickerPurposeApplyTransferDestination:
		d := &a.model.TransferDialog
		rn := []rune(path)
		d.Destination.Value = path
		d.Destination.Cursor = len(rn)
		d.Destination.Prefill = ""
		d.Destination.PrefillPending = false
		d.DestSubFocus = dialog.TransferDestSubFocusText
		a.closePathPicker()
	case dialog.PathPickerPurposeApplyFlattenDestination:
		d := &a.model.FlattenDialog
		rn := []rune(path)
		d.Destination.Value = path
		d.Destination.Cursor = len(rn)
		d.Destination.Prefill = ""
		d.Destination.PrefillPending = false
		d.DestSubFocus = dialog.FlattenDestSubFocusText
		a.closePathPicker()
	case dialog.PathPickerPurposeApplyFileDialogField:
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
	if a.tryBookmarkDialogShortcut(event) {
		return
	}
	if a.tryStandardDialogActions(event, a.activatePathPickerSelection, a.closePathPicker, nil) {
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
		if nf, ok := dialog.ListOKCancelNavFocusKey(st.Focus, event.Key()); ok {
			st.Focus = nf
		}
	case tcell.KeyBacktab:
		if nf, ok := dialog.ListOKCancelNavFocusKey(st.Focus, event.Key()); ok {
			st.Focus = nf
		}
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := dialog.ListOKCancelNavFocusKey(st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				dialog.EnsurePathPickerListScroll(st, a.pathPickerListRows())
			}
			break
		}
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.pathPickerListRows, func() {
			dialog.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		}) {
			break
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if handleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), a.pathPickerListRows, func() {
			dialog.EnsurePathPickerListScroll(st, a.pathPickerListRows())
		}) {
			break
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
func (a *App) pathPickerItemsHistoryAndBookmarks() ([]dialog.PathPickerItem, error) {
	passive := a.inactivePanel()
	active := a.activePanel()
	panelPath := active.PathString()
	home := a.model.UserHomeDir
	seen := make(map[string]struct{})
	var items []dialog.PathPickerItem

	for _, cp := range panel.MergeNavigationHistories(passive.History, active.History) {
		if pathpick.QueryLooksPathlike(cp) && pathEntryMissing(panelPath, home, cp) {
			continue
		}
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		items = append(items, dialog.PathPickerItem{
			Source:      "history",
			Path:        cp,
			PathMissing: pathEntryMissing(panelPath, home, cp),
		})
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
		items = append(items, dialog.PathPickerItem{
			Source:      marks[i].Origin.PathPickerSource(),
			Name:        marks[i].Name,
			Path:        cp,
			PathMissing: pathEntryMissing(panelPath, home, cp),
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

func (a *App) openPathPickerForFlatten() {
	a.transferDestValidate.Invalidate()
	items, err := a.pathPickerItemsHistoryAndBookmarks()
	if err != nil {
		a.setErrorMessage("Choose path", err)
		return
	}
	if len(items) == 0 {
		a.setTransientMessage("No paths in history or bookmarks", ui.MessageUrgencyInfo)
		return
	}
	a.model.PathPicker = dialog.PathPickerState{
		Open:       true,
		Title:      "Choose path",
		Purpose:    dialog.PathPickerPurposeApplyFlattenDestination,
		Query:      "",
		Items:      items,
		Focus:      0,
		Selected:   0,
		ListScroll: 0,
	}
	a.syncPathPickerRanks()
}

func (a *App) openPathPickerForTransfer() {
	a.transferDestValidate.Invalidate()
	items, err := a.pathPickerItemsHistoryAndBookmarks()
	if err != nil {
		a.setErrorMessage("Choose path", err)
		return
	}
	if len(items) == 0 {
		a.setTransientMessage("No paths in history or bookmarks", ui.MessageUrgencyInfo)
		return
	}
	a.model.PathPicker = dialog.PathPickerState{
		Open:       true,
		Title:      "Choose path",
		Purpose:    dialog.PathPickerPurposeApplyTransferDestination,
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
	a.model.PathPicker = dialog.PathPickerState{
		Open:           true,
		Title:          "Choose path",
		Purpose:        dialog.PathPickerPurposeApplyFileDialogField,
		FileFieldIndex: fieldIndex,
		Query:          "",
		Items:          items,
		Focus:          0,
		Selected:       0,
		ListScroll:     0,
	}
	a.syncPathPickerRanks()
}

func (a *App) armPathPickerValidateTimer() {
	if !a.model.PathPicker.Open {
		return
	}
	st := &a.model.PathPicker
	st.QueryPathCheckPending = true
	delay := time.Duration(a.config.UI.PathPickerValidateDelayMS) * time.Millisecond
	a.pathPickerValidate.Arm(delay, func() {
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

func (a *App) armTransferDestinationValidateTimer() {
	if !a.model.TransferDialog.Open || a.model.TransferDialog.Phase != dialog.TransferPhaseDestination {
		return
	}
	d := &a.model.TransferDialog
	d.DestPathCheckPending = true
	delay := time.Duration(a.config.UI.PathPickerValidateDelayMS) * time.Millisecond
	a.transferDestValidate.Arm(delay, func() {
		if !a.model.TransferDialog.Open || a.model.TransferDialog.Phase != dialog.TransferPhaseDestination {
			return
		}
		a.applyTransferDestinationPathValidation()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(transferDestValidatePayload{}))
	})
}

func (a *App) applyTransferDestinationPathValidation() {
	d := &a.model.TransferDialog
	if !d.Open || d.Phase != dialog.TransferPhaseDestination {
		return
	}
	d.DestPathCheckPending = false
	panel := a.activePanel()
	d.DestPathInvalid = pathpick.TypedDoesNotExist(panel.PathString(), a.model.UserHomeDir, d.Destination.Value)
	a.syncOpenPathInputsAfterFSChange()
}

func (a *App) armFlattenDestinationValidateTimer() {
	if !a.model.FlattenDialog.Open {
		return
	}
	d := &a.model.FlattenDialog
	d.DestPathCheckPending = true
	delay := time.Duration(a.config.UI.PathPickerValidateDelayMS) * time.Millisecond
	a.transferDestValidate.Arm(delay, func() {
		if !a.model.FlattenDialog.Open {
			return
		}
		a.applyFlattenDestinationPathValidation()
		_ = a.screen.PostEvent(tcell.NewEventInterrupt(transferDestValidatePayload{}))
	})
}

func (a *App) applyFlattenDestinationPathValidation() {
	d := &a.model.FlattenDialog
	if !d.Open {
		return
	}
	d.DestPathCheckPending = false
	panel := a.activePanel()
	d.DestPathInvalid = pathpick.TypedDoesNotExist(panel.PathString(), a.model.UserHomeDir, d.Destination.Value)
	a.syncOpenPathInputsAfterFSChange()
}
