// Package copiesview owns the Other copies view: the copies of one indexed file as a flat list to
// mark, copy, move and delete, beside a file-list panel that shows the directory of the cursor copy.
package copiesview

import (
	"path/filepath"
	"slices"

	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// Title names the feature in messages.
const Title = "Other copies"

// Host is the app shell surface the view needs.
type Host interface {
	host.MessageHost
	host.PanelNavigationHost
	CopiesMenuDefinitions() []menu.Definition
	BrowserMenuDefinitions() []menu.Definition
	Config() config.Config
}

// Deps wires the handler at app construction.
type Deps struct {
	Host  Host
	Model *ui.Model
}

// Handler owns the view's list state; the embedded panel is the app's (internal/app/copies_view.go).
type Handler struct {
	host  Host
	model *ui.Model
}

// New constructs the handler.
func New(d Deps) *Handler { return &Handler{host: d.Host, model: d.Model} }

// Open shows the copies of the file called origin; indexPanel: the lookup started from a
// fileindex:// panel.
func (h *Handler) Open(origin string, indexPanel bool, copies []fi.CopyFile) {
	files := make([]ui.UniqueFile, len(copies))
	for i, c := range copies {
		files[i] = ui.UniqueFile{Path: c.Path, Size: c.Size}
	}
	h.model.CopiesView = ui.CopiesViewState{Open: true, Origin: origin, IndexPanel: indexPanel, Files: files}
	h.model.ViewMode = ui.ViewCopies
	h.model.MenuDefinitions = h.host.CopiesMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndexDedup()
}

// Leave returns to the browser and forgets the copies.
func (h *Handler) Leave() {
	h.model.CopiesView = ui.CopiesViewState{}
	h.model.ViewMode = ui.ViewBrowser
	h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndex()
}

// EnsureSelectionVisible scrolls the cursor into the list's visibleRows.
func (h *Handler) EnsureSelectionVisible(visibleRows int) {
	st := &h.model.CopiesView
	st.Pane.EnsureSelectionVisible(len(st.Files), visibleRows)
}

// MoveSelection moves the cursor by delta rows (clamped).
func (h *Handler) MoveSelection(delta int) {
	st := &h.model.CopiesView
	if n := len(st.Files); n > 0 {
		st.Pane.Selected = min(max(st.Pane.Selected+delta, 0), n-1)
	}
}

// SelectEdge moves the cursor to the first or last row.
func (h *Handler) SelectEdge(last bool) {
	st := &h.model.CopiesView
	st.Pane.Selected = 0
	if last {
		st.Pane.Selected = max(len(st.Files)-1, 0)
	}
}

// SelectRow moves the cursor to row and gives the list focus (a click).
func (h *Handler) SelectRow(row int) {
	st := &h.model.CopiesView
	st.FocusPanel = false
	st.Pane.Selected = row
}

// SwitchPane moves focus between the list and the panel.
func (h *Handler) SwitchPane() {
	st := &h.model.CopiesView
	st.FocusPanel = !st.FocusPanel
	st.Pane.Filter = quickfilter.Filter{}
}

// FocusPanel gives the panel focus (a click).
func (h *Handler) FocusPanel() {
	st := &h.model.CopiesView
	st.FocusPanel = true
	st.Pane.Filter = quickfilter.Filter{}
}

// cursorFile is the copy under the cursor.
func (h *Handler) cursorFile() (ui.UniqueFile, bool) {
	st := &h.model.CopiesView
	if st.Pane.Selected < 0 || st.Pane.Selected >= len(st.Files) {
		return ui.UniqueFile{}, false
	}
	return st.Files[st.Pane.Selected], true
}

// CursorPath is the host path of the cursor copy (what view and edit open).
func (h *Handler) CursorPath() (string, bool) {
	f, ok := h.cursorFile()
	return f.Path, ok
}

// Target is the cursor copy's path as the embedded panel and the browser panels show it: a
// fileindex:// path when the lookup started from one.
func (h *Handler) Target() (path string, ok bool) {
	f, ok := h.cursorFile()
	if !ok {
		return "", false
	}
	loc, err := pathloc.ForPanel(f.Path, h.model.CopiesView.IndexPanel)
	if err != nil {
		return "", false
	}
	return loc.String(), true
}

// SelectToggleAndAdvance marks or unmarks the cursor copy and moves down.
func (h *Handler) SelectToggleAndAdvance() {
	if f, ok := h.cursorFile(); ok {
		st := &h.model.CopiesView
		st.SetMark(f, !st.Marked[f.Path])
		h.MoveSelection(1)
	}
}

// ClearMarks drops every mark.
func (h *Handler) ClearMarks() {
	st := &h.model.CopiesView
	st.Marked, st.MarkedBytes = nil, 0
}

// Sources are the copies an operation acts on: the marked ones, else the cursor copy.
func (h *Handler) Sources() []ui.UniqueFile {
	st := &h.model.CopiesView
	if marked := st.MarkedFiles(); len(marked) > 0 {
		return marked
	}
	if f, ok := h.cursorFile(); ok {
		return []ui.UniqueFile{f}
	}
	return nil
}

// OpenInPanel shows path in panelID: a directory itself, a file in its directory.
func (h *Handler) OpenInPanel(panelID int, path string, isDir bool) {
	name, err := host.NavigatePanelToEntry(h.host, panelID, path, isDir)
	if err != nil {
		h.host.SetTransientMessage("Open: "+err.Error(), ui.MessageUrgencyWarn)
		return
	}
	host.ReportOpenedInPanel(h.host, name, panelID)
}

// FilesQueued records an operation queued on paths: marks go, and with gone (a move, a delete) the
// copies leave the list; the cursor stays on the nearest row left.
func (h *Handler) FilesQueued(paths []string, gone bool) {
	st := &h.model.CopiesView
	h.ClearMarks()
	if !gone {
		return
	}
	st.Files = slices.DeleteFunc(slices.Clone(st.Files), func(f ui.UniqueFile) bool { return slices.Contains(paths, f.Path) })
	st.Pane.Selected = min(st.Pane.Selected, max(len(st.Files)-1, 0))
	if st.Pane.Filter.Query != "" {
		st.Pane.Filter.Rebuild(h.names(), h.filterOptions())
	}
}

// Type-to-jump filter over the list's names (the listFilterCtrl surface).

func (h *Handler) names() []string {
	files := h.model.CopiesView.Files
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = filepath.Base(f.Path)
	}
	return names
}

func (h *Handler) filterOptions() quickfilter.Options {
	f := h.host.Config().Filter
	return quickfilter.OptionsFrom(f.CaseInsensitive, f.CycleMatches)
}

func (h *Handler) applyFilter(query string) {
	st := &h.model.CopiesView
	if cur, ok := st.Pane.Filter.Apply(query, h.names(), h.filterOptions()); ok {
		st.Pane.Selected = cur
	}
}

// Filter is the list's quick filter.
func (h *Handler) Filter() quickfilter.Filter { return h.model.CopiesView.Pane.Filter }

// AppendFilterRune types r into the filter.
func (h *Handler) AppendFilterRune(r rune) {
	h.applyFilter(h.model.CopiesView.Pane.Filter.InsertRune(r))
}

// BackspaceFilter deletes the rune before the filter caret.
func (h *Handler) BackspaceFilter() {
	if next, changed := h.model.CopiesView.Pane.Filter.Backspace(); changed {
		h.applyFilter(next)
	}
}

// ClearFilter empties the query while the filter stays open.
func (h *Handler) ClearFilter() { h.applyFilter("") }

// CancelFilter closes the filter.
func (h *Handler) CancelFilter() { h.model.CopiesView.Pane.Filter = quickfilter.Filter{} }

// CycleFilterMatch steps the cursor through the matches, or moves plainly when nothing matches.
func (h *Handler) CycleFilterMatch(delta int) {
	st := &h.model.CopiesView
	if cur, ok := st.Pane.Filter.Cycle(st.Pane.Selected, delta, h.filterOptions()); ok {
		st.Pane.Selected = cur
		return
	}
	h.MoveSelection(delta)
}
