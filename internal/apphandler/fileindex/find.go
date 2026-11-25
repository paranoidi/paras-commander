package fileindex

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	apphost "github.com/paranoidi/paras-commander/internal/apphandler/host"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// FindPayload carries the outcome of one find-in-index query; Gen says which query it answers.
type FindPayload struct {
	Gen       int
	Results   []dialog.IndexFindResult
	Truncated bool
	Err       error
}

// OpenFindDialog opens the find-in-index dialog for panel panelID, a fileindex:// or local panel,
// once the index is known to open.
func (h *Handler) OpenFindDialog(panelID int) {
	h.WithIndex(func() { h.openFindDialog(panelID) })
}

func (h *Handler) openFindDialog(panelID int) {
	if !h.model.FileListFocused() {
		return
	}
	p := h.host.PanelByID(panelID)
	root, ok := pathloc.HostOf(p.PathString())
	if !ok {
		h.host.SetErrorMessage("Find in index", errNoHostPath)
		return
	}
	if st := &h.model.IndexFindDialog; st.Parked {
		if st.PanelID == panelID && st.Root == root {
			h.RestoreParkedFind()
			return
		}
		h.model.FindRelocate = dialog.FindRelocateState{Open: true, Focus: dialog.FindRelocateRestore, Index: true, PanelID: panelID, Root: st.Root}
		return
	}
	h.openFreshFind(panelID, p, root)
}

// RestoreParkedFind shows the parked Find in index as it was, wherever it was parked.
func (h *Handler) RestoreParkedFind() {
	st := &h.model.IndexFindDialog
	st.Parked, st.Open = false, true
}

// OpenFreshFind drops any Find in index (open or parked) and starts a new one on panelID.
func (h *Handler) OpenFreshFind(panelID int) {
	h.CloseFindDialog()
	h.openFindDialog(panelID)
}

func (h *Handler) openFreshFind(panelID int, p *panel.State, root string) {
	var selRoots []string
	for _, uri := range panel.PruneNestedPaths(p.SelectedDirectoryPaths()) {
		if hp, ok := pathloc.HostOf(uri); ok {
			selRoots = append(selRoots, hp)
		}
	}
	h.CloseFindDialog()
	adv := h.savedAdvanced
	if adv.Scope == dialog.IndexScopeSelected && len(selRoots) == 0 {
		adv.Scope = dialog.IndexScopeWhole
	}
	h.model.IndexFindDialog = dialog.IndexFindDialogState{
		ShowSelectedScope: len(selRoots) > 0, SelectionDirRoots: selRoots,
		Advanced:         adv,
		AdvancedMnemonic: dialog.FindAdvancedMnemonic(h.keysFind.BindingsForAction(keymap.ActionFindToggleAdvanced)),
		Gen:              h.model.IndexFindDialog.Gen, // keeps counting up, so a result of an earlier dialog is stale
		Open:             true, PanelID: panelID, FromIndexPanel: p.Path.Scheme() == pathloc.SchemeFileIndex, Root: root, Focus: dialog.IndexFindQuery,
	}
}

// CloseFindDialog closes the dialog, keeps its advanced section for the next one and drops any
// query still in flight.
func (h *Handler) CloseFindDialog() {
	if st := &h.model.IndexFindDialog; st.Open || st.Parked || h.findHiddenForView {
		h.savedAdvanced = st.Advanced
	}
	h.findHiddenForView = false
	h.stopFind()
	gen := h.model.IndexFindDialog.Gen + 1
	h.model.IndexFindDialog = dialog.IndexFindDialogState{Gen: gen} // a late result never matches
}

// stopFind drops the pending query's timer and cancels the one in flight.
func (h *Handler) stopFind() {
	if h.findTimer != nil {
		h.findTimer.Stop()
		h.findTimer = nil
	}
	if h.findCancel != nil {
		h.findCancel()
		h.findCancel = nil
	}
}

// scheduleFind runs the dialog's query after delay on a goroutine, cancelling the previous query;
// an empty query clears the list.
func (h *Handler) scheduleFind(delay time.Duration) {
	st := &h.model.IndexFindDialog
	h.stopFind()
	st.Gen++
	st.Err = ""
	if strings.TrimSpace(st.Query) == "" {
		st.Results, st.Truncated, st.Searching = nil, false, false
		st.Selected, st.ListScroll = 0, 0
		return
	}
	st.Searching = true
	gen, root, limit := st.Gen, st.Root, h.config.UI.Find.MaxResults
	q := fi.FindQuery{
		Text: st.Query, Mode: st.Advanced.Mode, OnlyDirs: st.OnlyDirs, OnlyFiles: st.OnlyFiles,
		MaxDepth: st.MaxDepth, IncludeHidden: st.IncludeHidden,
	}
	whole := st.Advanced.Scope == dialog.IndexScopeWhole
	if whole {
		root = "/"
	}
	if st.Advanced.Scope == dialog.IndexScopeSelected {
		q.Roots = slices.Clone(st.SelectionDirRoots)
	}
	home := h.model.UserHomeDir
	ctx, cancel := context.WithCancel(context.Background())
	h.findCancel = cancel
	h.findTimer = time.AfterFunc(delay, func() {
		found, truncated, err := h.index.Find(ctx, root, q, limit)
		if ctx.Err() != nil {
			return // a newer query or the closed dialog took over
		}
		res := make([]dialog.IndexFindResult, len(found))
		for i, f := range found {
			rel := primitive.PathWithHomeTilde(f.Path, home)
			if !whole {
				rel = pathloc.RelOrSelf(root, f.Path)
			}
			res[i] = dialog.IndexFindResult{Path: f.Path, Rel: rel, IsDir: f.IsDir, Size: f.Size}
		}
		h.post(FindPayload{Gen: gen, Results: res, Truncated: truncated, Err: err})
	})
}

// HandleFind applies a finished query unless the dialog closed or the query changed since.
func (h *Handler) HandleFind(p FindPayload) {
	st := &h.model.IndexFindDialog
	if (!st.Open && !st.Parked && !h.findHiddenForView) || p.Gen != st.Gen {
		return
	}
	st.Searching = false
	st.Results, st.Truncated, st.Selected, st.ListScroll = p.Results, p.Truncated, 0, 0
	if p.Err != nil {
		st.Err = p.Err.Error()
	}
}

// DialogSpec is the open dialog's widget spec for the current screen.
func (h *Handler) DialogSpec() dlg.Spec {
	return dialog.IndexFindDialogView(&h.model.IndexFindDialog, h.viewContext())
}

func (h *Handler) viewContext() dialog.IndexFindViewContext {
	w, hh := h.screen.Size()
	return dialog.IndexFindViewContext{
		Layout: h.host.LayoutForTerminalSize(w, hh), Styles: h.host.Styles(),
		ShowIcons: h.model.UseNerdfontIcons, ScrollbarStyle: h.model.PanelScrollbar,
	}
}

func (h *Handler) listRows() int {
	return max(dialog.IndexFindListRows(h.model.IndexFindDialog, h.viewContext().Layout), 1)
}

func (h *Handler) queryWidth() int {
	w, hh := h.screen.Size()
	return max(dlg.InputTextWidth(w, hh, h.DialogSpec(), dialog.IndexFindQuery), 1)
}

func (h *Handler) debounce() time.Duration {
	return time.Duration(h.config.UI.Find.QueryDebounceMS) * time.Millisecond
}

// HandleFindKey handles one key while the dialog is open: the shared [dialog.find] actions, mark
// toggling, the widget engine, then query editing.
func (h *Handler) HandleFindKey(ev *tcell.EventKey) {
	st := &h.model.IndexFindDialog
	if id, ok := h.keys.Lookup(ev); ok && findctrl.IsParkKey(id) {
		st.Open, st.Parked = false, true // hidden with its state kept; the opener key shows it again
		return
	}
	if h.keysFind != nil {
		if id, ok := h.keysFind.Lookup(ev); ok && h.runFindAction(id) {
			return
		}
	}
	// Plain space is query text here, even though it is globally bound to select-toggle.
	if id, ok := h.keys.Lookup(ev); ok && id == keymap.ActionPanelSelectToggle && !keymap.IsPlainPrintableRune(ev) {
		h.runFindAction(id)
		return
	}
	if id, ok := h.keys.Lookup(ev); ok && id == keymap.ActionPanelPinToggle {
		h.pinCursor()
		return
	}
	if ev.Key() == tcell.KeyInsert {
		h.runFindAction(keymap.ActionPanelSelectToggle)
		return
	}
	if st.Focus == dialog.IndexFindDepth {
		switch ev.Key() {
		case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyUp, tcell.KeyDown:
			h.commitDepth()
		case tcell.KeyLeft:
			h.commitDepth()
			st.Focus = dialog.IndexFindQuery
			return
		}
	}
	if e, ok := dlg.HandleKey(h.DialogSpec(), &st.Focus, ev); ok {
		h.ApplyFindEvent(e)
		return
	}
	if st.Focus == dialog.IndexFindDepth {
		h.handleDepthKey(ev)
		return
	}
	if st.Focus == dialog.IndexFindQuery {
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, h.queryWidth(), func() {
			st.Selected, st.ListScroll = 0, 0
			h.scheduleFind(h.debounce())
		})
		scrollquery.HandleKey(h.keysDialogInput, ev, true, edit)
	}
}

// runFindAction runs a [dialog.find] action this dialog supports (false: not handled). The
// advanced section is the match mode and the scope: toggle shows it, cycle steps the match mode,
// reset returns both to their defaults.
func (h *Handler) runFindAction(id string) bool {
	st := &h.model.IndexFindDialog
	switch id {
	case keymap.ActionPanelSelectToggle:
		h.toggleMark()
	case keymap.ActionPanelPinToggle:
		h.pinCursor()
	case keymap.ActionFindView:
		h.viewCursor()
	case keymap.ActionFindEdit:
		h.editCursor()
	case keymap.ActionFindUnselectAll:
		st.Marked, st.MarkedDirs, st.MarkedSizes = nil, nil, nil
	case keymap.ActionFindSelectAll:
		h.mark(st.Results...)
	case keymap.ActionOpenInPrimary:
		h.openCursorIn(h.model.FirstSlotPanel())
	case keymap.ActionOpenInSecondary:
		h.openCursorIn(h.model.SecondSlotPanel())
	case keymap.ActionFindSelectGroup:
		h.host.OpenGroupSelectDialog(findctrl.GroupSelectModeSelect)
	case keymap.ActionFindUnselectGroup:
		h.host.OpenGroupSelectDialog(findctrl.GroupSelectModeUnselect)
	case keymap.ActionFindSelectParentDirs:
		h.selectParentDirs()
	case keymap.ActionFindToggleAdvanced:
		h.toggleAdvanced()
	case keymap.ActionFindCycleMatchMode:
		st.Apply(dialog.IndexFindAdvModeID(st.Advanced.Mode.Next()))
		h.scheduleFind(0)
	case keymap.ActionFindResetAdvanced:
		st.ResetAdvanced()
		h.scheduleFind(0)
	default:
		return false
	}
	return true
}

// ApplyFindEvent acts on a widget event (key or click).
func (h *Handler) ApplyFindEvent(ev dlg.Event) {
	st := &h.model.IndexFindDialog
	switch {
	case ev.Kind == dlg.EventCaret:
		if ev.ID == dialog.IndexFindQuery {
			st.QueryCursor = ev.Index
		}
	case ev.ID == dialog.IndexFindList && ev.Kind == dlg.EventListMove:
		st.Selected, st.ListScroll = ev.Index, ev.Offset
	case ev.ID == "":
	case ev.ID == dialog.IndexFindAdvancedToggle:
		h.toggleAdvanced()
	case ev.ID == dialog.IndexFindDepth:
		h.commitDepth()
		st.Focus = dialog.IndexFindQuery
	case ev.ID == dialog.IndexFindQuery, ev.ID == dlg.OK:
		h.ActivateFindOK()
	case ev.ID == dlg.Cancel:
		h.CloseFindDialog()
	default:
		st.Apply(ev.ID)
		h.scheduleFind(0)
	}
}

func (h *Handler) cursorResult() (dialog.IndexFindResult, bool) {
	st := &h.model.IndexFindDialog
	if st.Selected < 0 || st.Selected >= len(st.Results) {
		return dialog.IndexFindResult{}, false
	}
	return st.Results[st.Selected], true
}

// mark adds results to the marks, dropping marks they conflict with (a directory and its content).
func (h *Handler) mark(results ...dialog.IndexFindResult) {
	st := &h.model.IndexFindDialog
	if st.Marked == nil {
		st.Marked, st.MarkedDirs, st.MarkedSizes = map[string]bool{}, map[string]bool{}, map[string]int64{}
	}
	paths := make([]string, 0, len(results))
	for _, r := range results {
		st.MarkedDirs[r.Path] = r.IsDir
		st.MarkedSizes[r.Path] = r.Size
		paths = append(paths, r.Path)
	}
	if panel.ApplySelectionAdds(st.Marked, paths, func(p string) bool { return st.MarkedDirs[p] }) {
		h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
	}
}

func (h *Handler) toggleMark() {
	st := &h.model.IndexFindDialog
	r, ok := h.cursorResult()
	if !ok || st.Focus != dialog.IndexFindQuery {
		return
	}
	if st.Marked[r.Path] {
		delete(st.Marked, r.Path)
	} else {
		h.mark(r)
	}
	if st.Selected < len(st.Results)-1 {
		st.Selected++
		dialog.EnsureIndexFindListScroll(st, h.listRows())
	}
}

// ActivateFindOK is OK / Enter: it merges the marks into the panel selection, else shows the
// result under the cursor in the panel.
func (h *Handler) ActivateFindOK() {
	if h.model.IndexFindDialog.MarkedCount() > 0 {
		h.applyMarks()
		return
	}
	panelID := h.model.IndexFindDialog.PanelID
	if _, ok := h.navigateTo(panelID); !ok {
		return
	}
	h.model.FocusPanel(panelID)
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseFindDialog()
}

func (h *Handler) applyMarks() {
	st := &h.model.IndexFindDialog
	p := h.host.PanelByID(st.PanelID)
	var paths []string
	isDir := map[string]bool{}
	for host, on := range st.Marked {
		loc, err := pathloc.ForPanel(host, st.FromIndexPanel)
		if err != nil || !on {
			continue
		}
		uri := loc.String()
		if p.SelectedPaths[uri] {
			continue
		}
		paths = append(paths, uri)
		isDir[uri] = st.MarkedDirs[host]
	}
	conflicts := false
	if len(paths) > 0 {
		conflicts = p.BulkAddSelections(paths, func(s string) bool { return isDir[s] })
	}
	h.model.FocusPanel(st.PanelID)
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseFindDialog()
	apphost.ReportSelectionAdded(h.host, len(paths), conflicts)
}

// navigateTo shows the result under the cursor in panelID (a directory itself, a file in its
// parent with the file highlighted); it does not close the dialog.
func (h *Handler) navigateTo(panelID int) (string, bool) {
	r, ok := h.cursorResult()
	if !ok {
		return "", false
	}
	dir, name := r.Path, ""
	if !r.IsDir {
		dir, name = pathloc.Dir(r.Path), filepath.Base(r.Path)
	}
	if !apphost.ShowInPanel(h.host, panelID, dir, name, h.model.IndexFindDialog.FromIndexPanel, "Find in index") {
		return "", false
	}
	return filepath.Base(r.Path), true
}

func (h *Handler) openCursorIn(panelID int) {
	if name, ok := h.navigateTo(panelID); ok {
		apphost.ReportOpenedInPanel(h.host, name, panelID)
	}
}

// viewCursor opens the result under the cursor, the real file, in the fullscreen viewer.
func (h *Handler) viewCursor() {
	r, ok := h.cursorResult()
	if !ok {
		return
	}
	if r.IsDir {
		h.host.SetTransientMessage("View: not a file", ui.MessageUrgencyWarn)
		return
	}
	st := &h.model.IndexFindDialog
	st.Open, h.findHiddenForView = false, true
	if err := h.host.OpenFullscreenFilePreviewAt(r.Path, false); err != nil {
		st.Open, h.findHiddenForView = true, false
		h.host.SetTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
	}
}

// ReopenFindAfterPreviewClose shows the find-in-index dialog again, as it was, when the
// just-closed fullscreen viewer was opened from it (find.view); no-op otherwise.
func (h *Handler) ReopenFindAfterPreviewClose() {
	if !h.findHiddenForView {
		return
	}
	h.findHiddenForView = false
	h.model.IndexFindDialog.Open = true
}

// editCursor opens the result under the cursor in the external editor and keeps the dialog open;
// a directory says "Edit: not a file".
func (h *Handler) editCursor() {
	r, ok := h.cursorResult()
	if !ok {
		return
	}
	if r.IsDir {
		h.host.SetTransientMessage("Edit: not a file", ui.MessageUrgencyWarn)
		return
	}
	h.host.EditFile(r.Path)
}

// pinCursor pins or unpins the result under the cursor by its fileindex:// path, the path
// pinning from a fileindex:// panel gives.
func (h *Handler) pinCursor() {
	r, ok := h.cursorResult()
	if !ok {
		return
	}
	loc, err := pathloc.FileIndex(r.Path)
	if err != nil {
		h.host.SetErrorMessage("Find in index", err)
		return
	}
	h.host.PinTogglePath(filepath.Base(r.Path), loc.String(), r.IsDir)
}

// toggleAdvanced shows or hides the advanced section. Showing it moves focus to the selected
// match mode; hiding it moves focus that was inside the section back to the query.
func (h *Handler) toggleAdvanced() {
	st := &h.model.IndexFindDialog
	st.Advanced.Expanded = !st.Advanced.Expanded
	if st.Advanced.Expanded {
		st.Focus = dialog.IndexFindAdvModeID(st.Advanced.Mode)
	} else if st.AdvancedFocused() {
		st.Focus = dialog.IndexFindQuery
	}
	dialog.EnsureIndexFindListScroll(st, h.listRows())
}

// commitDepth applies the depth edit buffer (one digit 1-9, "" = unlimited) and re-runs the query.
// A depth counts from the panel directory, so it moves the whole-index scope to the current
// directory.
func (h *Handler) commitDepth() {
	st := &h.model.IndexFindDialog
	next := 0
	if st.DepthInput != "" {
		next = int(st.DepthInput[0] - '0')
	}
	if next == st.MaxDepth {
		return
	}
	st.MaxDepth = next
	if next > 0 && st.Advanced.Scope == dialog.IndexScopeWhole {
		st.Advanced.Scope = dialog.IndexScopeCurrent
	}
	h.scheduleFind(0)
}

// handleDepthKey edits the depth field like Find's: 1-9 sets it, 0 / Backspace / Delete clear it,
// and either returns focus to the query.
func (h *Handler) handleDepthKey(ev *tcell.EventKey) {
	st := &h.model.IndexFindDialog
	switch ev.Key() {
	case tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete:
		st.DepthInput = ""
	case tcell.KeyRune:
		if ev.Modifiers() != tcell.ModNone {
			return
		}
		switch r := ev.Rune(); {
		case r >= '1' && r <= '9':
			st.DepthInput = string(r)
		case r == '0':
			st.DepthInput = ""
		default:
			return
		}
	default:
		return
	}
	h.commitDepth()
	st.Focus = dialog.IndexFindQuery
}

// markedOrCursorURIs is the marked results (else the cursor row) as fileindex:// paths.
func (h *Handler) markedOrCursorURIs() []string {
	st := &h.model.IndexFindDialog
	hosts := make([]string, 0, len(st.Marked))
	for p, on := range st.Marked {
		if on {
			hosts = append(hosts, p)
		}
	}
	if r, ok := h.cursorResult(); len(hosts) == 0 && ok {
		hosts = append(hosts, r.Path)
	}
	var uris []string
	for _, p := range hosts {
		if loc, err := pathloc.FileIndex(p); err == nil {
			uris = append(uris, loc.String())
		}
	}
	return uris
}

// selectParentDirs replaces the panel selection with the parent directories of the marked
// results, else of the cursor row (Find's select-parent-dirs).
func (h *Handler) selectParentDirs() {
	st := &h.model.IndexFindDialog
	paths := h.markedOrCursorURIs()
	if len(paths) == 0 {
		h.host.SetTransientMessage("No selection", ui.MessageUrgencyInfo)
		return
	}
	p := h.host.PanelByID(st.PanelID)
	dirs := p.ParentDirsExcludingSelf(paths)
	if len(dirs) == 0 {
		h.host.SetTransientMessage("Already in the containing directory", ui.MessageUrgencyInfo)
		return
	}
	message, conflicts := p.SelectDirs(dirs)
	h.model.FocusPanel(st.PanelID)
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseFindDialog()
	if conflicts {
		h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
		return
	}
	h.host.SetTransientMessage(message, ui.MessageUrgencyInfo)
}

// groupMatches are the results req selects: its basename / path matches within the files / dirs
// filter. The list is capped at max_results, so matching runs synchronously.
func groupMatches(results []dialog.IndexFindResult, req findctrl.GroupSelectRequest) ([]dialog.IndexFindResult, error) {
	match, err := findctrl.NewPathMatcher(req)
	if err != nil {
		return nil, err
	}
	var out []dialog.IndexFindResult
	for _, r := range results {
		if !req.Scope.Targets(r.IsDir) || !match(r.Path) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// CountGroupMatches counts the results req matches whose marked state equals marked, as files and
// directories (the group-select dialog's live preview).
func (h *Handler) CountGroupMatches(req findctrl.GroupSelectRequest, marked bool) (files, dirs int) {
	st := &h.model.IndexFindDialog
	if req.Empty() {
		return 0, 0
	}
	matched, err := groupMatches(st.Results, req)
	if err != nil {
		return 0, 0
	}
	for _, r := range matched {
		if st.Marked[r.Path] != marked {
			continue
		}
		if r.IsDir {
			dirs++
		} else {
			files++
		}
	}
	return files, dirs
}

// ApplyGroupSelect marks or unmarks the results req matches.
func (h *Handler) ApplyGroupSelect(req findctrl.GroupSelectRequest) {
	st := &h.model.IndexFindDialog
	if req.Empty() {
		return
	}
	matched, err := groupMatches(st.Results, req)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyCritical)
		return
	}
	if len(matched) == 0 {
		h.host.SetTransientMessage("No matches", ui.MessageUrgencyWarn)
		return
	}
	if req.Mode == findctrl.GroupSelectModeSelect {
		h.mark(matched...)
		h.host.SetTransientMessage("Selected matching "+req.Describe(), ui.MessageUrgencyInfo)
		return
	}
	for _, r := range matched {
		delete(st.Marked, r.Path)
	}
	h.host.SetTransientMessage("Unselected matching "+req.Describe(), ui.MessageUrgencyInfo)
}
