package dialog

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// InvalidateTransferDestValidate stops any pending debounced transfer/flatten destination-path
// validation and bumps its generation, so an in-flight callback is ignored. Used when a dialog
// sharing that debouncer closes.
func (h *Handler) InvalidateTransferDestValidate() {
	h.transferDestValidate.Invalidate()
}

// PathPickerValidateArmed reports whether the path-picker filter's debounced validation is
// currently scheduled (test support).
func (h *Handler) PathPickerValidateArmed() bool {
	return h.pathPickerValidate.Armed()
}

// PathPickerValidateGeneration returns the path-picker filter's debounce generation (test
// support: bumped by each Arm/Invalidate, so tests can assert a check was (in)validated).
func (h *Handler) PathPickerValidateGeneration() uint64 {
	return h.pathPickerValidate.Generation()
}

// ClosePathPicker closes the fuzzy path/history picker and, if it was opened to apply a
// transfer or flatten destination, re-arms that dialog's destination validation timer (the
// picker's own validate timer was invalidated while it had focus).
func (h *Handler) ClosePathPicker() {
	purpose := h.model.PathPicker.Purpose
	h.pathPickerValidate.Invalidate()
	h.model.PathPicker = dialog.PathPickerState{}
	if h.model.TransferDialog.Open && h.model.TransferDialog.Phase == dialog.TransferPhaseDestination &&
		purpose == dialog.PathPickerPurposeApplyTransferDestination {
		h.ArmTransferDestinationValidateTimer()
	}
	if h.model.FlattenDialog.Open && purpose == dialog.PathPickerPurposeApplyFlattenDestination {
		h.ArmFlattenDestinationValidateTimer()
	}
}

// SyncPathPickerRanks re-filters and re-ranks the path picker's item list against the current
// query, clamping selection and list scroll.
func (h *Handler) SyncPathPickerRanks() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	lines := make([]string, len(st.Items))
	for i, e := range st.Items {
		lines[i] = e.SearchLine()
	}
	cfg := h.host.Config()
	st.Ranked, st.MatchRanges = h.host.SyncFilteredListRanks(lines, st.Query, len(st.Items), cfg.Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&st.Selected, len(st.Ranked))
	dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows())
}

// SyncPathPickerCompletion updates the path picker query's filesystem completion ghost text.
func (h *Handler) SyncPathPickerCompletion() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	p := h.host.ActivePanel()
	cfg := h.host.Config()
	c, ok := pathpick.SuggestAtCursor(p.PathString(), h.model.UserHomeDir, st.Query, st.QueryCursor, cfg.Panels.ShowHidden)
	if !ok {
		st.QueryCompletionSuffix = ""
		st.QueryCompletionIsDir = false
		h.SyncPathPickerScroll()
		return
	}
	st.QueryCompletionSuffix = c.Suffix
	st.QueryCompletionIsDir = c.IsDir
	h.SyncPathPickerScroll()
}

// SyncPathPickerScroll re-clamps the query input's cursor/scroll to keep the caret visible.
func (h *Handler) SyncPathPickerScroll() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	width := h.PathPickerQueryWidth()
	valueLen := len([]rune(st.Query))
	suffixLen := len([]rune(st.QueryCompletionSuffix))
	st.QueryCursor, st.QueryScroll = dialog.EnsurePathInputScroll(valueLen, st.QueryCursor, st.QueryScroll, width, suffixLen)
}

// pathPickerScrollToCaret re-clamps the query input's cursor/scroll after AcceptPathPickerCompletion
// inserts a suggestion, keeping the caret visible.
func (h *Handler) pathPickerScrollToCaret() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	width := h.PathPickerQueryWidth()
	valueLen := len([]rune(st.Query))
	suffixLen := len([]rune(st.QueryCompletionSuffix))
	st.QueryCursor, st.QueryScroll = dialog.EnsurePathInputScroll(valueLen, st.QueryCursor, st.QueryScroll, width, suffixLen)
}

// AcceptPathPickerCompletion inserts the current completion suggestion at the cursor.
func (h *Handler) AcceptPathPickerCompletion() {
	st := &h.model.PathPicker
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
	h.pathPickerScrollToCaret()

	h.SyncPathPickerRanks()
	h.SyncPathPickerCompletion()
	h.ArmPathPickerValidateTimer()
	st.Selected = 0
	dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows())
}

// PathPickerListRows returns how many rows the path picker's fuzzy list currently shows.
func (h *Handler) PathPickerListRows() int {
	termW, termH := h.screen.Size()
	layout := h.host.LayoutForTerminalSize(termW, termH)
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

func (h *Handler) activatePathPickerSelection() {
	st := &h.model.PathPicker
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
		p := h.host.ActivePanel()
		if err := h.host.NavigatePanelToPath(h.model.ActivePanel, path, ""); err != nil {
			h.host.SetErrorMessage("Bookmark", err)
			return
		}
		p.EnsureCursorVisible(h.host.ActiveViewportRows())
		h.ClosePathPicker()
		h.host.SetTransientMessage(path, ui.MessageUrgencyInfo)
	case dialog.PathPickerPurposeApplyTransferDestination:
		d := &h.model.TransferDialog
		rn := []rune(path)
		d.Destination.Value = path
		d.Destination.Cursor = len(rn)
		d.Destination.Prefill = ""
		d.Destination.PrefillPending = false
		d.DestSubFocus = dialog.TransferDestSubFocusText
		h.ClosePathPicker()
	case dialog.PathPickerPurposeApplyFlattenDestination:
		d := &h.model.FlattenDialog
		rn := []rune(path)
		d.Destination.Value = path
		d.Destination.Cursor = len(rn)
		d.Destination.Prefill = ""
		d.Destination.PrefillPending = false
		d.DestSubFocus = dialog.FlattenDestSubFocusText
		h.ClosePathPicker()
	case dialog.PathPickerPurposeApplyFileDialogField:
		idx := st.FileFieldIndex
		if idx < 0 || idx >= len(h.model.FileDialog.Fields) {
			h.ClosePathPicker()
			return
		}
		f := &h.model.FileDialog.Fields[idx]
		f.Value = path
		f.Cursor = len([]rune(path))
		f.Prefill = ""
		f.PrefillPending = false
		f.PickerFocused = false
		h.ClosePathPicker()
	default:
		h.ClosePathPicker()
	}
}

// pathPickerNavFocus applies list+OK+Cancel navigation, hiding OK for navigate/bookmark.
func pathPickerNavFocus(purpose dialog.PathPickerPurpose, focus int, key tcell.Key) (int, bool) {
	return dialog.ListDialogForm{HideOK: purpose == dialog.PathPickerPurposeNavigate}.MoveFocus(focus, key)
}

// HandlePathPickerKey routes a key event for the open fuzzy path/history picker.
func (h *Handler) HandlePathPickerKey(event *tcell.EventKey) {
	st := &h.model.PathPicker
	if h.TryBookmarkDialogShortcut(event) {
		return
	}
	if dialog.TryStandardDialogActions(event, h.activatePathPickerSelection, h.ClosePathPicker, nil) {
		return
	}

	if st.Focus == 0 && h.host.HandlePathPickerScrollingQueryKey(event) {
		return
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.ClosePathPicker()
	case tcell.KeyEnter:
		switch st.Focus {
		case 2:
			h.ClosePathPicker()
		default:
			h.activatePathPickerSelection()
		}
	case tcell.KeyTab:
		if st.Focus == 0 && st.QueryCompletionSuffix != "" {
			h.AcceptPathPickerCompletion()
			return
		}
		if nf, ok := pathPickerNavFocus(st.Purpose, st.Focus, event.Key()); ok {
			st.Focus = nf
		}
	case tcell.KeyBacktab:
		if nf, ok := pathPickerNavFocus(st.Purpose, st.Focus, event.Key()); ok {
			st.Focus = nf
		}
	case tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := pathPickerNavFocus(st.Purpose, st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows())
			}
			break
		}
		if h.host.HandleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), h.PathPickerListRows, func() {
			dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows())
		}) {
			break
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if h.host.HandleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), h.PathPickerListRows, func() {
			dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows())
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
		switch dialog.DialogButtonRune(event.Rune()) {
		case dialog.ButtonRuneOK:
			h.activatePathPickerSelection()
		case dialog.ButtonRuneCancel:
			h.ClosePathPicker()
		case dialog.ButtonRuneToggle:
			switch st.Focus {
			case 1:
				h.activatePathPickerSelection()
			case 2:
				h.ClosePathPicker()
			}
		}
	}
}

// PathPickerQueryWidth returns the visible width of the query input row.
// Mirrors the layout in drawPathPickerDialog: rect.Width - 4 with rect width = 78
// clamped to layout.Width - 4.
func (h *Handler) PathPickerQueryWidth() int {
	termW, _ := h.screen.Size()
	width := 78
	if width > termW-4 {
		width = termW - 4
	}
	if width < 36 {
		width = 36
	}
	return width - 4
}

// PathPickerItemsHistoryAndBookmarks returns merged passive-first panel histories plus
// bookmarks (deduped by cleaned path), each with a display line for fuzzy matching.
func (h *Handler) PathPickerItemsHistoryAndBookmarks() ([]dialog.PathPickerItem, error) {
	passive := h.host.InactivePanel()
	active := h.host.ActivePanel()
	panelPath := active.PathString()
	home := h.model.UserHomeDir
	seen := make(map[string]struct{})
	var items []dialog.PathPickerItem

	for _, cp := range panel.MergeNavigationHistories(passive.History, active.History) {
		if pathpick.QueryLooksPathlike(cp) && PathEntryMissing(panelPath, home, cp) {
			continue
		}
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		items = append(items, dialog.PathPickerItem{
			Source:      "history",
			Path:        cp,
			PathMissing: PathEntryMissing(panelPath, home, cp),
		})
	}

	cfg := h.host.Config()
	marks, err := bookmarks.LoadAll(cfg.Bookmarks.File, h.model.UserHomeDir)
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
			PathMissing: PathEntryMissing(panelPath, home, cp),
		})
	}
	return items, nil
}

// PathEntryMissing reports whether path (typed relative to panelPath/home) currently resolves
// to an existing filesystem entry. Shared by the path picker and the bookmarks dialog.
func PathEntryMissing(panelPath, home, path string) bool {
	if strings.HasPrefix(path, "sftp://") {
		return pathpick.TypedDoesNotExist(panelPath, home, path)
	}
	_, err := os.Lstat(path)
	return err != nil
}

// OpenPathPickerForFlatten opens the fuzzy path/history picker to apply the flatten dialog's
// destination field.
func (h *Handler) OpenPathPickerForFlatten() {
	h.transferDestValidate.Invalidate()
	items, err := h.PathPickerItemsHistoryAndBookmarks()
	if err != nil {
		h.host.SetErrorMessage("Choose path", err)
		return
	}
	if len(items) == 0 {
		h.host.SetTransientMessage("No paths in history or bookmarks", ui.MessageUrgencyInfo)
		return
	}
	h.model.PathPicker = dialog.PathPickerState{
		Open:       true,
		Title:      "Choose path",
		Purpose:    dialog.PathPickerPurposeApplyFlattenDestination,
		Query:      "",
		Items:      items,
		Focus:      0,
		Selected:   0,
		ListScroll: 0,
	}
	h.SyncPathPickerRanks()
}

// OpenPathPickerForTransfer opens the fuzzy path/history picker to apply the transfer
// (copy/move) dialog's destination field.
func (h *Handler) OpenPathPickerForTransfer() {
	h.transferDestValidate.Invalidate()
	items, err := h.PathPickerItemsHistoryAndBookmarks()
	if err != nil {
		h.host.SetErrorMessage("Choose path", err)
		return
	}
	if len(items) == 0 {
		h.host.SetTransientMessage("No paths in history or bookmarks", ui.MessageUrgencyInfo)
		return
	}
	h.model.PathPicker = dialog.PathPickerState{
		Open:       true,
		Title:      "Choose path",
		Purpose:    dialog.PathPickerPurposeApplyTransferDestination,
		Query:      "",
		Items:      items,
		Focus:      0,
		Selected:   0,
		ListScroll: 0,
	}
	h.SyncPathPickerRanks()
}

// OpenPathPickerForFileField opens the fuzzy path/history picker to apply a generic file
// dialog's path-picker field (fieldIndex into FileDialogState.Fields).
func (h *Handler) OpenPathPickerForFileField(fieldIndex int) {
	items, err := h.PathPickerItemsHistoryAndBookmarks()
	if err != nil {
		h.host.SetErrorMessage("Choose path", err)
		return
	}
	if len(items) == 0 {
		h.host.SetTransientMessage("No paths in history or bookmarks", ui.MessageUrgencyInfo)
		return
	}
	h.model.PathPicker = dialog.PathPickerState{
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
	h.SyncPathPickerRanks()
}

// ArmPathPickerValidateTimer (re)arms the debounced "does the typed query resolve to an
// existing path" check for the open path picker.
func (h *Handler) ArmPathPickerValidateTimer() {
	if !h.model.PathPicker.Open {
		return
	}
	st := &h.model.PathPicker
	st.QueryPathCheckPending = true
	cfg := h.host.Config()
	delay := time.Duration(cfg.UI.PathPickerValidateDelayMS) * time.Millisecond
	h.pathPickerValidate.Arm(delay, func() {
		if !h.model.PathPicker.Open {
			return
		}
		h.ApplyPathPickerPathValidation()
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(PathPickerValidatePayload{}))
	})
}

// ApplyPathPickerPathValidation applies the debounced path-existence check to the open path
// picker's query.
func (h *Handler) ApplyPathPickerPathValidation() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	st.QueryPathCheckPending = false
	p := h.host.ActivePanel()
	st.QueryPathInvalid = pathpick.TypedDoesNotExist(p.PathString(), h.model.UserHomeDir, st.Query)
	h.SyncOpenPathInputsAfterFSChange()
}

// ArmTransferDestinationValidateTimer (re)arms the debounced "does the typed destination
// resolve to an existing path" check for the open transfer (copy/move) dialog.
func (h *Handler) ArmTransferDestinationValidateTimer() {
	if !h.model.TransferDialog.Open || h.model.TransferDialog.Phase != dialog.TransferPhaseDestination {
		return
	}
	d := &h.model.TransferDialog
	d.DestPathCheckPending = true
	cfg := h.host.Config()
	delay := time.Duration(cfg.UI.PathPickerValidateDelayMS) * time.Millisecond
	h.transferDestValidate.Arm(delay, func() {
		if !h.model.TransferDialog.Open || h.model.TransferDialog.Phase != dialog.TransferPhaseDestination {
			return
		}
		h.ApplyTransferDestinationPathValidation()
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(TransferDestValidatePayload{}))
	})
}

// ApplyTransferDestinationPathValidation applies the debounced path-existence check to the
// open transfer dialog's destination field, and updates which panel(s) it currently targets.
func (h *Handler) ApplyTransferDestinationPathValidation() {
	d := &h.model.TransferDialog
	if !d.Open || d.Phase != dialog.TransferPhaseDestination {
		return
	}
	d.DestPathCheckPending = false
	p := h.host.ActivePanel()
	d.DestPathInvalid = pathpick.TypedDoesNotExist(p.PathString(), h.model.UserHomeDir, d.Destination.Value)
	h.updateDestinationTargetPanels(p.PathString(), d.Destination.Value)
	h.SyncOpenPathInputsAfterFSChange()
}

// ArmFlattenDestinationValidateTimer (re)arms the debounced "does the typed destination
// resolve to an existing path" check for the open flatten dialog.
func (h *Handler) ArmFlattenDestinationValidateTimer() {
	if !h.model.FlattenDialog.Open {
		return
	}
	d := &h.model.FlattenDialog
	d.DestPathCheckPending = true
	cfg := h.host.Config()
	delay := time.Duration(cfg.UI.PathPickerValidateDelayMS) * time.Millisecond
	h.transferDestValidate.Arm(delay, func() {
		if !h.model.FlattenDialog.Open {
			return
		}
		h.ApplyFlattenDestinationPathValidation()
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(TransferDestValidatePayload{}))
	})
}

// ApplyFlattenDestinationPathValidation applies the debounced path-existence check to the
// open flatten dialog's destination field, and updates which panel(s) it currently targets.
func (h *Handler) ApplyFlattenDestinationPathValidation() {
	d := &h.model.FlattenDialog
	if !d.Open {
		return
	}
	d.DestPathCheckPending = false
	p := h.host.ActivePanel()
	d.DestPathInvalid = pathpick.TypedDoesNotExist(p.PathString(), h.model.UserHomeDir, d.Destination.Value)
	h.updateDestinationTargetPanels(p.PathString(), d.Destination.Value)
	h.SyncOpenPathInputsAfterFSChange()
}

// updateDestinationTargetPanels resolves typed (the Copy/Move/Flatten destination text,
// relative to panelPath) and marks whichever visible panel(s) it points at so drawPanel
// can paint that panel's border with theme.PanelTargetFrame.
func (h *Handler) updateDestinationTargetPanels(panelPath, typed string) {
	typed = strings.TrimSpace(typed)
	if typed == "" {
		h.model.DestinationTargetPrimary = false
		h.model.DestinationTargetSecondary = false
		return
	}
	abs := filepath.Clean(pathpick.ResolveQuery(panelPath, h.model.UserHomeDir, typed))
	h.model.DestinationTargetPrimary = filepath.Clean(h.model.Primary.PathString()) == abs
	h.model.DestinationTargetSecondary = filepath.Clean(h.model.Secondary.PathString()) == abs
}
