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
	h.transferDestValidateSeq++
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
	h.pathPickerValidateSeq++
	h.pathPickerMissingGen++
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

// resyncPathPickerCompletion recomputes filesystem completion for the path picker's query.
func (h *Handler) resyncPathPickerCompletion() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	h.syncPathCompletion(&st.Completion, st.Query, st.QueryCursor, false)
	h.SyncPathPickerScroll()
}

// SyncPathPickerCompletion updates the path picker query's filesystem completion dropdown state.
func (h *Handler) SyncPathPickerCompletion() {
	h.resyncPathPickerCompletion()
}

// SyncPathPickerScroll re-clamps the query input's cursor/scroll to keep the caret visible.
func (h *Handler) SyncPathPickerScroll() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	width := h.PathPickerQueryWidth()
	valueLen := len([]rune(st.Query))
	suffixLen := len([]rune(st.Completion.GhostSuffix(st.Query)))
	st.QueryCursor, st.QueryScroll = dialog.EnsurePathInputScroll(valueLen, st.QueryCursor, st.QueryScroll, width, suffixLen)
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
	dialogHeight := 7 + listH
	if dialogHeight > layout.Height-2 {
		listH = layout.Height - 2 - 7
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
		h.ClosePathPicker()
	case dialog.PathPickerPurposeApplyFlattenDestination:
		d := &h.model.FlattenDialog
		rn := []rune(path)
		d.Destination.Value = path
		d.Destination.Cursor = len(rn)
		d.Destination.Prefill = ""
		d.Destination.PrefillPending = false
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
	if st.Focus == 0 {
		if handled, accepted := dialog.HandlePathCompletionKey(event, &st.Completion, &st.Query, &st.QueryCursor); handled {
			if accepted {
				h.resyncPathPickerCompletion()
				h.SyncPathPickerRanks()
				h.ArmPathPickerValidateTimer()
				st.Selected = 0
				dialog.EnsurePathPickerListScroll(st, h.PathPickerListRows())
			}
			return
		}
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

// PathPickerItemsHistory returns merged passive-first panel histories (deduped by cleaned
// path). PathMissing starts false for every item; startPathPickerMissingScan fills it in
// asynchronously after the picker opens.
func (h *Handler) PathPickerItemsHistory() ([]dialog.PathPickerItem, error) {
	passive := h.host.InactivePanel()
	active := h.host.ActivePanel()
	seen := make(map[string]struct{})
	var items []dialog.PathPickerItem

	for _, cp := range panel.MergeNavigationHistories(passive.History, active.History) {
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		items = append(items, dialog.PathPickerItem{Path: cp})
	}
	return items, nil
}

// PathPickerItemsBookmarks returns fzf-marks and GTK bookmarks (deduped by cleaned path).
// PathMissing starts false for every item; startPathPickerMissingScan fills it in
// asynchronously after the picker opens.
func (h *Handler) PathPickerItemsBookmarks() ([]dialog.PathPickerItem, error) {
	home := h.model.UserHomeDir
	cfg := h.host.Config()
	marks, err := bookmarks.LoadAll(cfg.Bookmarks.File, home)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(marks))
	items := make([]dialog.PathPickerItem, 0, len(marks))
	for i := range marks {
		cp := filepath.Clean(marks[i].Path)
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		items = append(items, dialog.PathPickerItem{
			Source: marks[i].Origin.PathPickerSource(),
			Name:   marks[i].Name,
			Path:   cp,
		})
	}
	return items, nil
}

// PathPickerItemsPinned returns pinned directories only (files excluded — the Pinned selector
// is for picking a destination), deduped by cleaned path. PathMissing starts false for every
// item; startPathPickerMissingScan fills it in asynchronously after the picker opens.
func (h *Handler) PathPickerItemsPinned() ([]dialog.PathPickerItem, error) {
	seen := make(map[string]struct{}, len(h.model.PinnedItems))
	items := make([]dialog.PathPickerItem, 0, len(h.model.PinnedItems))
	for _, p := range h.model.PinnedItems {
		if !p.IsDir {
			continue
		}
		cp := filepath.Clean(p.Path)
		if _, ok := seen[cp]; ok {
			continue
		}
		seen[cp] = struct{}{}
		items = append(items, dialog.PathPickerItem{Path: cp})
	}
	return items, nil
}

// startPathPickerMissingScan bumps the path picker's missing-scan generation and starts a
// background scan of every open item's path, so ApplyPathPickerMissing can later fill in
// PathMissing without blocking dialog open on stats (see StartPathsMissingScan).
func (h *Handler) startPathPickerMissingScan() {
	st := &h.model.PathPicker
	h.pathPickerMissingGen++
	paths := make([]string, len(st.Items))
	for i, it := range st.Items {
		paths[i] = it.Path
	}
	panelPath := h.host.ActivePanel().PathString()
	StartPathsMissingScan(h.screen, "picker", h.pathPickerMissingGen, panelPath, h.model.UserHomeDir, paths)
}

// ApplyPathPickerMissing applies a background missing-path scan's result to the open path
// picker's items. Ignored if the picker has since closed or a newer scan has been started.
func (h *Handler) ApplyPathPickerMissing(p PathsMissingPayload) {
	st := &h.model.PathPicker
	if !st.Open || p.Gen != h.pathPickerMissingGen {
		return
	}
	for i := range st.Items {
		st.Items[i].PathMissing = p.Missing[st.Items[i].Path]
	}
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

// pathPickerListKind selects bookmarks-only vs history-only items for apply-to-field pickers.
type pathPickerListKind int

const (
	pathPickerListBookmarks pathPickerListKind = iota
	pathPickerListHistory
	pathPickerListPinned
	pathPickerListAll
)

func (h *Handler) openPathPickerApply(purpose dialog.PathPickerPurpose, kind pathPickerListKind, fileFieldIndex int) {
	if kind == pathPickerListBookmarks {
		h.openBookmarkPathPicker(purpose, fileFieldIndex)
		return
	}
	if kind == pathPickerListAll {
		h.openAllPathPicker(purpose, fileFieldIndex)
		return
	}
	var (
		items                     []dialog.PathPickerItem
		err                       error
		title, emptyMsg, errTitle string
	)
	switch kind {
	case pathPickerListHistory:
		title, emptyMsg, errTitle = "History", "No paths in history", "History"
		items, err = h.PathPickerItemsHistory()
	case pathPickerListPinned:
		title, emptyMsg, errTitle = "Pinned", "No pinned directories", "Pinned"
		items, err = h.PathPickerItemsPinned()
	}
	if err != nil {
		h.host.SetErrorMessage(errTitle, err)
		return
	}
	if len(items) == 0 {
		h.host.SetTransientMessage(emptyMsg, ui.MessageUrgencyInfo)
		return
	}
	h.model.PathPicker = dialog.PathPickerState{
		Open:           true,
		Title:          title,
		Purpose:        purpose,
		FileFieldIndex: fileFieldIndex,
		Query:          "",
		Items:          items,
		Focus:          0,
		Selected:       0,
		ListScroll:     0,
	}
	h.SyncPathPickerRanks()
	h.startPathPickerMissingScan()
}

// openAllPathPicker opens the combined picker: pinned and history items synchronously, then
// bookmarks merged in by applyBookmarkLoad once the worker read completes.
func (h *Handler) openAllPathPicker(purpose dialog.PathPickerPurpose, fileFieldIndex int) {
	pinned, _ := h.PathPickerItemsPinned()
	history, _ := h.PathPickerItemsHistory()
	seen := make(map[string]struct{}, len(pinned)+len(history))
	var items []dialog.PathPickerItem
	for _, it := range pinned {
		seen[it.Path] = struct{}{}
		it.Source = "pinned"
		items = append(items, it)
	}
	for _, it := range history {
		if _, ok := seen[it.Path]; ok {
			continue
		}
		it.Source = "history"
		items = append(items, it)
	}
	h.pathPickerMissingGen++
	gen := h.pathPickerMissingGen
	h.model.PathPicker = dialog.PathPickerState{
		Open:           true,
		Title:          "All paths",
		Purpose:        purpose,
		FileFieldIndex: fileFieldIndex,
		Items:          items,
	}
	h.SyncPathPickerRanks()
	h.startBookmarkListLoad(gen)
}

// openBookmarkPathPicker opens the bookmarks path picker immediately and fills it via
// startBookmarkListLoad / ApplyBookmarkIO. bookmarks.LoadAll runs on a worker, not the
// UI goroutine; history and pinned lists stay synchronous in openPathPickerApply.
func (h *Handler) openBookmarkPathPicker(purpose dialog.PathPickerPurpose, fileFieldIndex int) {
	h.pathPickerMissingGen++
	gen := h.pathPickerMissingGen
	h.model.PathPicker = dialog.PathPickerState{
		Open:           true,
		Title:          "Bookmarks",
		Purpose:        purpose,
		FileFieldIndex: fileFieldIndex,
		Query:          "",
		Focus:          0,
		Selected:       0,
		ListScroll:     0,
	}
	h.SyncPathPickerRanks()
	h.startBookmarkListLoad(gen)
}

// OpenPathPickerForFlatten opens a path picker of the given kind to apply the flatten
// dialog's destination field.
func (h *Handler) OpenPathPickerForFlatten(kind pathPickerListKind) {
	h.transferDestValidate.Invalidate()
	h.openPathPickerApply(dialog.PathPickerPurposeApplyFlattenDestination, kind, 0)
}

// OpenPathPickerForTransfer opens a path picker of the given kind to apply the transfer
// (copy/move) dialog's destination field.
func (h *Handler) OpenPathPickerForTransfer(kind pathPickerListKind) {
	h.transferDestValidate.Invalidate()
	h.openPathPickerApply(dialog.PathPickerPurposeApplyTransferDestination, kind, 0)
}

// OpenPathPickerForFileField opens a path picker of the given kind to apply a generic file
// dialog's path-picker field (fieldIndex into FileDialogState.Fields).
func (h *Handler) OpenPathPickerForFileField(fieldIndex int, kind pathPickerListKind) {
	h.openPathPickerApply(dialog.PathPickerPurposeApplyFileDialogField, kind, fieldIndex)
}

// ArmPathPickerValidateTimer (re)arms the debounced "does the typed query resolve to an
// existing path" check for the open path picker. The timer goroutine only computes an
// immutable result; ApplyPathPickerValidatePayload mutates ui.Model on the event loop.
func (h *Handler) ArmPathPickerValidateTimer() {
	if !h.model.PathPicker.Open {
		return
	}
	st := &h.model.PathPicker
	st.QueryPathCheckPending = true
	h.pathPickerValidateSeq++
	seq := h.pathPickerValidateSeq
	query := st.Query
	panelPath := h.host.ActivePanel().PathString()
	home := h.model.UserHomeDir
	existsFn := h.pathExistsFn
	screen := h.screen
	cfg := h.host.Config()
	delay := time.Duration(cfg.UI.PathPickerValidateDelayMS) * time.Millisecond
	h.pathPickerValidate.Arm(delay, func() {
		invalid := pathExists(existsFn, panelPath, home, query)
		if screen == nil {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(PathPickerValidatePayload{
			Gen:     seq,
			Query:   query,
			Invalid: invalid,
		}))
	})
}

// ApplyPathPickerPathValidation runs the path-picker existence check for the current query
// on the caller goroutine and applies it (used by tests that drive validation synchronously).
func (h *Handler) ApplyPathPickerPathValidation() {
	st := &h.model.PathPicker
	if !st.Open {
		return
	}
	p := h.host.ActivePanel()
	h.ApplyPathPickerValidatePayload(PathPickerValidatePayload{
		Gen:     h.pathPickerValidateSeq,
		Query:   st.Query,
		Invalid: pathExists(h.pathExistsFn, p.PathString(), h.model.UserHomeDir, st.Query),
	})
}

// ApplyPathPickerValidatePayload applies a background path-picker existence check unless the
// picker closed, a newer check was armed, or the typed query no longer matches.
func (h *Handler) ApplyPathPickerValidatePayload(d PathPickerValidatePayload) {
	st := &h.model.PathPicker
	if !st.Open || d.Gen != h.pathPickerValidateSeq || d.Query != st.Query {
		return
	}
	st.QueryPathCheckPending = false
	st.QueryPathInvalid = d.Invalid
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
	h.armTransferDestValidate(d.Destination.Value, false)
}

// ApplyTransferDestinationPathValidation runs the transfer destination existence check for
// the current field value on the caller goroutine and applies it (used by tests that drive
// validation synchronously).
func (h *Handler) ApplyTransferDestinationPathValidation() {
	d := &h.model.TransferDialog
	if !d.Open || d.Phase != dialog.TransferPhaseDestination {
		return
	}
	p := h.host.ActivePanel()
	h.ApplyTransferDestValidatePayload(TransferDestValidatePayload{
		Gen:     h.transferDestValidateSeq,
		Query:   d.Destination.Value,
		Invalid: pathExists(h.pathExistsFn, p.PathString(), h.model.UserHomeDir, d.Destination.Value),
	})
}

// ArmFlattenDestinationValidateTimer (re)arms the debounced "does the typed destination
// resolve to an existing path" check for the open flatten dialog.
func (h *Handler) ArmFlattenDestinationValidateTimer() {
	if !h.model.FlattenDialog.Open {
		return
	}
	d := &h.model.FlattenDialog
	d.DestPathCheckPending = true
	h.armTransferDestValidate(d.Destination.Value, true)
}

// ApplyFlattenDestinationPathValidation runs the flatten destination existence check for
// the current field value on the caller goroutine and applies it (used by tests that drive
// validation synchronously).
func (h *Handler) ApplyFlattenDestinationPathValidation() {
	d := &h.model.FlattenDialog
	if !d.Open {
		return
	}
	p := h.host.ActivePanel()
	h.ApplyTransferDestValidatePayload(TransferDestValidatePayload{
		Gen:     h.transferDestValidateSeq,
		Query:   d.Destination.Value,
		Invalid: pathExists(h.pathExistsFn, p.PathString(), h.model.UserHomeDir, d.Destination.Value),
		Flatten: true,
	})
}

// ApplyTransferDestValidatePayload applies a background transfer/flatten destination existence
// check unless that dialog closed, a newer check was armed, or the typed destination changed.
func (h *Handler) ApplyTransferDestValidatePayload(d TransferDestValidatePayload) {
	if d.Flatten {
		st := &h.model.FlattenDialog
		if !st.Open || d.Gen != h.transferDestValidateSeq || d.Query != st.Destination.Value {
			return
		}
		st.DestPathCheckPending = false
		st.DestPathInvalid = d.Invalid
		p := h.host.ActivePanel()
		h.updateDestinationTargetPanels(p.PathString(), st.Destination.Value)
		h.SyncOpenPathInputsAfterFSChange()
		return
	}
	st := &h.model.TransferDialog
	if !st.Open || st.Phase != dialog.TransferPhaseDestination ||
		d.Gen != h.transferDestValidateSeq || d.Query != st.Destination.Value {
		return
	}
	st.DestPathCheckPending = false
	st.DestPathInvalid = d.Invalid
	p := h.host.ActivePanel()
	h.updateDestinationTargetPanels(p.PathString(), st.Destination.Value)
	h.SyncOpenPathInputsAfterFSChange()
}

func (h *Handler) armTransferDestValidate(query string, flatten bool) {
	h.transferDestValidateSeq++
	seq := h.transferDestValidateSeq
	panelPath := h.host.ActivePanel().PathString()
	home := h.model.UserHomeDir
	existsFn := h.pathExistsFn
	screen := h.screen
	cfg := h.host.Config()
	delay := time.Duration(cfg.UI.PathPickerValidateDelayMS) * time.Millisecond
	h.transferDestValidate.Arm(delay, func() {
		invalid := pathExists(existsFn, panelPath, home, query)
		if screen == nil {
			return
		}
		_ = screen.PostEvent(tcell.NewEventInterrupt(TransferDestValidatePayload{
			Gen:     seq,
			Query:   query,
			Invalid: invalid,
			Flatten: flatten,
		}))
	})
}

func pathExists(fn func(panelPath, home, raw string) bool, panelPath, home, raw string) bool {
	if fn != nil {
		return fn(panelPath, home, raw)
	}
	return pathpick.TypedDoesNotExist(panelPath, home, raw)
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
