package fileindex

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
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

// OpenFindDialog opens the find-in-index dialog for the fileindex:// panel panelID (jump: the
// directories-only "Jump to directory" form), once the index is known to open.
func (h *Handler) OpenFindDialog(panelID int, jump bool) {
	h.withIndex(func() { h.openFindDialog(panelID, jump) })
}

func (h *Handler) openFindDialog(panelID int, jump bool) {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	root, err := pathloc.IndexHostPath(h.host.PanelByID(panelID).Path)
	if err != nil {
		h.host.SetErrorMessage("Find in index", err)
		return
	}
	var selRoots []string
	if !jump {
		for _, uri := range panel.PruneNestedPaths(h.host.PanelByID(panelID).SelectedDirectoryPaths()) {
			if loc, err := pathloc.Parse(uri); err == nil {
				if hp, err := pathloc.IndexHostPath(loc); err == nil {
					selRoots = append(selRoots, hp)
				}
			}
		}
	}
	h.CloseFindDialog()
	h.model.IndexFindDialog = dialog.IndexFindDialogState{
		ShowSearchSelectionsOption: len(selRoots) > 0, SelectionDirRoots: selRoots,
		Gen:  h.model.IndexFindDialog.Gen, // keeps counting up, so a result of an earlier dialog is stale
		Open: true, PanelID: panelID, Root: root, Focus: dialog.IndexFindQuery,
		JumpMode: jump, OnlyDirs: jump,
	}
}

// CloseFindDialog closes the dialog and drops any query still in flight.
func (h *Handler) CloseFindDialog() {
	if h.findTimer != nil {
		h.findTimer.Stop()
		h.findTimer = nil
	}
	gen := h.model.IndexFindDialog.Gen + 1
	h.model.IndexFindDialog = dialog.IndexFindDialogState{Gen: gen} // a late result never matches
}

// scheduleFind runs the dialog's query after delay on a goroutine; an empty query clears the list.
func (h *Handler) scheduleFind(delay time.Duration) {
	st := &h.model.IndexFindDialog
	if h.findTimer != nil {
		h.findTimer.Stop()
		h.findTimer = nil
	}
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
		Text: st.Query, Words: st.Words, OnlyDirs: st.OnlyDirs, OnlyFiles: st.OnlyFiles,
		MaxDepth: st.MaxDepth, IncludeHidden: st.IncludeHidden,
	}
	if st.SearchOnlySelections {
		q.Roots = slices.Clone(st.SelectionDirRoots)
	}
	h.findTimer = time.AfterFunc(delay, func() {
		found, truncated, err := h.index.Find(root, q, limit)
		res := make([]dialog.IndexFindResult, len(found))
		for i, f := range found {
			res[i] = dialog.IndexFindResult{Path: f.Path, Rel: relTo(root, f.Path), IsDir: f.IsDir, Size: f.Size}
		}
		h.post(FindPayload{Gen: gen, Results: res, Truncated: truncated, Err: err})
	})
}

func relTo(root, path string) string {
	if rel, err := filepath.Rel(root, path); err == nil {
		return rel
	}
	return path
}

// HandleFind applies a finished query unless the dialog closed or the query changed since.
func (h *Handler) HandleFind(p FindPayload) {
	st := &h.model.IndexFindDialog
	if !st.Open || p.Gen != st.Gen {
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
	return max(dialog.IndexFindListRows(h.viewContext().Layout), 1)
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

// runFindAction runs a [dialog.find] action this dialog supports; the other find.* actions are
// swallowed, anything else is not handled (false).
func (h *Handler) runFindAction(id string) bool {
	st := &h.model.IndexFindDialog
	switch id {
	case keymap.ActionPanelSelectToggle, keymap.ActionFindView, keymap.ActionFindUnselectAll,
		keymap.ActionFindSelectAll, keymap.ActionFindSelectGroup, keymap.ActionFindUnselectGroup,
		keymap.ActionFindSelectParentDirs:
		if st.JumpMode {
			return true // marking and viewing do not apply to jump mode
		}
	}
	switch id {
	case keymap.ActionPanelSelectToggle:
		h.toggleMark()
	case keymap.ActionPanelPinToggle:
		h.pinCursor()
	case keymap.ActionFindView:
		h.viewCursor()
	case keymap.ActionFindUnselectAll:
		st.Marked, st.MarkedDirs, st.MarkedSizes = nil, nil, nil
	case keymap.ActionFindSelectAll:
		h.mark(st.Results...)
	case keymap.ActionOpenInPrimary:
		h.openCursorIn(ui.PrimaryPanel, "primary")
	case keymap.ActionOpenInSecondary:
		h.openCursorIn(ui.SecondaryPanel, "secondary")
	case keymap.ActionFindSelectGroup:
		h.host.OpenGroupSelectDialog(findctrl.GroupSelectModeSelect)
	case keymap.ActionFindUnselectGroup:
		h.host.OpenGroupSelectDialog(findctrl.GroupSelectModeUnselect)
	case keymap.ActionFindSelectParentDirs:
		h.selectParentDirs()
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
	h.model.ActivePanel = panelID
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseFindDialog()
}

func (h *Handler) applyMarks() {
	st := &h.model.IndexFindDialog
	p := h.host.PanelByID(st.PanelID)
	var paths []string
	isDir := map[string]bool{}
	for host, on := range st.Marked {
		loc, err := pathloc.FileIndex(host)
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
	h.model.ActivePanel = st.PanelID
	h.model.ActiveSubFocus = ui.SubFocusFileList
	h.CloseFindDialog()
	switch {
	case len(paths) == 0:
		h.host.SetTransientMessage("No new selections", ui.MessageUrgencyInfo)
	case conflicts:
		h.host.SetTransientMessage("Removed conflicting selections", ui.MessageUrgencyWarn)
	default:
		h.host.SetTransientMessage(fmt.Sprintf("Added %d to selection", len(paths)), ui.MessageUrgencyInfo)
	}
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
		dir, name = filepath.Dir(r.Path), filepath.Base(r.Path)
	}
	loc, err := pathloc.FileIndex(dir)
	if err == nil {
		err = h.host.NavigatePanelToPath(panelID, loc.String(), name)
	}
	if err != nil {
		h.host.SetErrorMessage("Find in index", err)
		return "", false
	}
	h.host.PanelByID(panelID).EnsureCursorVisible(h.host.PanelViewportRows(panelID))
	return filepath.Base(r.Path), true
}

func (h *Handler) openCursorIn(panelID int, which string) {
	if name, ok := h.navigateTo(panelID); ok {
		h.host.SetTransientMessage(fmt.Sprintf("Opened %s in %s panel", name, which), ui.MessageUrgencyInfo)
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
	h.CloseFindDialog()
	if err := h.host.OpenFullscreenFilePreviewAt(r.Path, false); err != nil {
		h.host.SetTransientMessage("View: "+err.Error(), ui.MessageUrgencyWarn)
	}
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

// commitDepth applies the depth edit buffer (one digit 1-9, "" = unlimited) and re-runs the query.
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
	h.model.ActivePanel = st.PanelID
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
		if (req.FilesOnly && r.IsDir) || (req.DirsOnly && !r.IsDir) || !match(r.Path) {
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
