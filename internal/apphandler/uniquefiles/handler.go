// Package uniquefiles owns the full-screen Find unique files view: the indexed files with no copy
// outside a scope of directories, as a tree to mark, copy, move, delete and hash.
package uniquefiles

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/quickfilter"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// Title names the feature in messages.
const Title = "Find unique files"

// errMixedSelection refuses a selection of both files and directories: the scope is either the
// selected directories or the panel directory, and a mix does not say which.
var errMixedSelection = errors.New("selection mixes files and directories")

const (
	mixedSelectionMessage   = Title + ": select only directories or only files"
	indexRootNothingOutside = "The index root has nothing outside it"
)

// Host is the app shell surface the view needs.
type Host interface {
	host.PanelNavigationHost
	PanelByID(panelID int) *panel.State
	SetTransientMessage(text string, urgency ui.MessageUrgency)
	SetErrorMessage(title string, err error)
	// AddTaskJob queues a task job working on sources.
	AddTaskJob(task *jobs.Task, sources []pathloc.Path)
	// EnqueueDeleteJob queues a delete of paths from disk, which the index follows; with
	// removeEmptyDirs it also removes the directories that leaves empty.
	EnqueueDeleteJob(paths []string, removeEmptyDirs bool)
	UniqueMenuDefinitions() []menu.Definition
	BrowserMenuDefinitions() []menu.Definition
	Config() config.Config
}

// Deps wires the handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model
	Index  *fi.Index
}

// Handler owns the view.
type Handler struct {
	host   Host
	screen tcell.Screen
	model  *ui.Model
	index  *fi.Index
	// lookupGen identifies the latest lookup: a result carrying another is stale.
	lookupGen int
	// pendingDelete holds the files of a confirmed delete while the empty-directories question is
	// open; its answer queues them (FinishDelete).
	pendingDelete []string
}

// New constructs the handler.
func New(d Deps) *Handler {
	return &Handler{host: d.Host, screen: d.Screen, model: d.Model, index: d.Index}
}

// LookupPayload carries the outcome of one index lookup.
type LookupPayload struct {
	Gen    int
	Result fi.UniqueResult
	Err    error
}

// HashedPayload wakes the event loop when a hash job of this view ended.
type HashedPayload struct{ Root string }

func (h *Handler) post(payload any) {
	if h.screen != nil {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(payload))
	}
}

// HandleWake applies one of this handler's wake payloads; it reports false for any other.
func (h *Handler) HandleWake(data any) bool {
	switch p := data.(type) {
	case LookupPayload:
		h.handleLookup(p)
	case HashedPayload:
		if st := &h.model.UniqueView; st.Loaded && st.Root == p.Root {
			h.lookup()
		}
	default:
		return false
	}
	return true
}

// Open shows the view for the active panel's scope: what is kept when the scope is the same, else
// a new lookup, which replaces it.
func (h *Handler) Open() {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	root, scope, err := h.scope(h.host.PanelByID(h.model.ActivePanel))
	switch {
	case errors.Is(err, errMixedSelection):
		h.host.SetTransientMessage(mixedSelectionMessage, ui.MessageUrgencyCritical)
		return
	case err != nil:
		h.host.SetErrorMessage(Title, err)
		return
	case slices.Contains(scope, "/"):
		h.host.SetTransientMessage(indexRootNothingOutside, ui.MessageUrgencyWarn)
		return
	}
	st := &h.model.UniqueView
	if (st.Loaded || st.Loading) && st.Root == root && slices.Equal(st.Scope, scope) {
		h.show()
		return
	}
	h.model.UniqueView = ui.UniqueViewState{Root: root, Scope: scope}
	h.model.UniqueList = nil
	h.show()
	h.lookup()
}

func (h *Handler) show() {
	h.model.ViewMode = ui.ViewUnique
	h.model.MenuDefinitions = h.host.UniqueMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndexDedup()
}

// Leave returns to the browser and keeps the results for the action that opened the view.
func (h *Handler) Leave() {
	h.model.ViewMode = ui.ViewBrowser
	h.model.MenuDefinitions = h.host.BrowserMenuDefinitions()
	h.model.Menu.ActiveMenu = menu.DefaultIndex()
}

// Refresh looks the scope up again, keeping marks, open directories and the cursor where they
// still apply.
func (h *Handler) Refresh() { h.lookup() }

func (h *Handler) lookup() {
	st := &h.model.UniqueView
	st.Loading = true
	h.lookupGen++
	gen, scope := h.lookupGen, slices.Clone(st.Scope)
	go func() {
		res, err := h.index.UniqueFiles(scope)
		h.post(LookupPayload{Gen: gen, Result: res, Err: err})
	}()
}

func (h *Handler) handleLookup(p LookupPayload) {
	st := &h.model.UniqueView
	if p.Gen != h.lookupGen || !st.Loading {
		return
	}
	st.Loading = false
	if p.Err != nil {
		h.host.SetErrorMessage(Title, p.Err)
		return
	}
	cursor := h.cursorID()
	st.Loaded = true
	st.Files = make([]ui.UniqueFile, len(p.Result.Files))
	present := make(map[string]int64, len(st.Files))
	for i, f := range p.Result.Files {
		st.Files[i] = ui.UniqueFile{Path: f.Path, Size: f.Size, Unverified: p.Result.Unverified[f.Path]}
		present[f.Path] = f.Size
	}
	st.ToHash = p.Result.ToHash
	h.keepMarksOf(present)
	h.rebuild(cursor)
	if len(p.Result.NotIndexed) > 0 {
		h.host.SetTransientMessage("Not in the index: "+strings.Join(p.Result.NotIndexed, ", "), ui.MessageUrgencyWarn)
	}
}

// keepMarksOf drops the marks of files no longer listed and recounts the marked bytes.
func (h *Handler) keepMarksOf(present map[string]int64) {
	st := &h.model.UniqueView
	st.MarkedBytes = 0
	for p := range st.Marked {
		size, ok := present[p]
		if !ok {
			delete(st.Marked, p)
			continue
		}
		st.MarkedBytes += size
	}
	if _, ok := present[st.RangeAnchor]; !ok {
		st.RangeAnchor = ""
	}
}

// rebuild recomputes the rows after the files, the collapsed set or the hidden toggle changed:
// directories new to the tree start collapsed and the cursor stays on cursorID when it still shows.
func (h *Handler) rebuild(cursorID string) {
	st := &h.model.UniqueView
	ids := ui.UniqueExpandableIDs(st)
	ui.DedupCollapseNewIDs(&st.Pane, st.PrevExpandable, ids)
	st.PrevExpandable = ids
	h.syncRows(cursorID)
}

func (h *Handler) syncRows(cursorID string) {
	st := &h.model.UniqueView
	rows := ui.UniqueRows(st)
	h.model.UniqueList = rows
	if cursorID != "" {
		if i := slices.IndexFunc(rows, func(r ui.UniqueRow) bool { return r.ID == cursorID }); i >= 0 {
			st.Pane.Selected = i
		}
	}
	st.Pane.Selected = min(st.Pane.Selected, max(len(rows)-1, 0))
	if st.Pane.Filter.Query != "" {
		st.Pane.Filter.Rebuild(rowNames(rows), h.filterOptions())
	}
}

func (h *Handler) cursorID() string {
	if r, ok := h.cursorRow(); ok {
		return r.ID
	}
	return ""
}

func (h *Handler) cursorRow() (ui.UniqueRow, bool) {
	st, rows := &h.model.UniqueView, h.model.UniqueList
	if st.Pane.Selected < 0 || st.Pane.Selected >= len(rows) {
		return ui.UniqueRow{}, false
	}
	return rows[st.Pane.Selected], true
}

// scope is the panel directory as the index records it and the directories to check: the selected
// directories, else the panel directory (a selection of only files does not change it).
func (h *Handler) scope(p *panel.State) (root string, scope []string, err error) {
	if root, err = fi.IndexPath(p.PathString()); err != nil {
		return "", nil, err
	}
	entries, err := p.SelectedEntries(true, h.selectedEntry)
	if err != nil {
		return "", nil, err
	}
	var dirs []string
	files := 0
	for _, e := range entries {
		if !e.ResolvesToDir() {
			files++
			continue
		}
		host, err := fi.IndexPath(e.Path)
		if err != nil {
			return "", nil, err
		}
		dirs = append(dirs, host)
	}
	switch {
	case len(dirs) > 0 && files > 0:
		return "", nil, errMixedSelection
	case len(dirs) > 0:
		return root, panel.PruneNestedPaths(dirs), nil
	}
	return root, []string{root}, nil
}

// selectedEntry resolves a selection off the panel listing: a fileindex:// path from the index,
// where it may no longer exist on disk.
func (h *Handler) selectedEntry(path string) (localfs.Entry, error) {
	loc, err := pathloc.Parse(path)
	if err != nil {
		return localfs.Entry{}, err
	}
	if loc.Scheme() != pathloc.SchemeFileIndex {
		return localfs.EntryFromPath(path)
	}
	e, err := fi.NewBackend(h.index).Stat(context.Background(), loc)
	if err != nil {
		return localfs.Entry{}, err
	}
	return localfs.Entry{Name: loc.Base(), Path: path, Type: fsbackend.LocalTypeFromBackend(e.Type)}, nil
}

// EnsureSelectionVisible scrolls the cursor into the tree's visibleRows.
func (h *Handler) EnsureSelectionVisible(visibleRows int) {
	h.model.UniqueView.Pane.EnsureSelectionVisible(len(h.model.UniqueList), visibleRows)
}

// MoveSelection moves the cursor by delta rows (clamped).
func (h *Handler) MoveSelection(delta int) {
	st, n := &h.model.UniqueView, len(h.model.UniqueList)
	if n > 0 {
		st.Pane.Selected = min(max(st.Pane.Selected+delta, 0), n-1)
	}
}

// SelectEdge moves the cursor to the first or last row.
func (h *Handler) SelectEdge(last bool) {
	st := &h.model.UniqueView
	st.Pane.Selected = 0
	if last {
		st.Pane.Selected = max(len(h.model.UniqueList)-1, 0)
	}
}

// SelectRow moves the cursor to row and gives the tree focus (a click).
func (h *Handler) SelectRow(row int) {
	st := &h.model.UniqueView
	st.FocusPanel = false
	st.Pane.Selected = row
}

// MoveToAdjacentDir moves the cursor to the previous or next directory row.
func (h *Handler) MoveToAdjacentDir(delta int) {
	st, rows := &h.model.UniqueView, h.model.UniqueList
	for i := st.Pane.Selected + delta; i >= 0 && i < len(rows); i += delta {
		if rows[i].Value.IsDir {
			st.Pane.Selected = i
			return
		}
	}
}

// ExpandOrDescend opens a collapsed directory in place; on an open one it steps to the first row
// inside. File rows do nothing.
func (h *Handler) ExpandOrDescend() {
	r, ok := h.cursorRow()
	if !ok || !r.Value.IsDir {
		return
	}
	st := &h.model.UniqueView
	if st.Pane.Collapsed[r.ID] {
		st.Pane.SetCollapsed(r.ID, false)
		h.syncRows(r.ID)
		return
	}
	h.MoveSelection(1)
}

// CollapseOrParent closes an open directory, else jumps to the row's parent directory.
func (h *Handler) CollapseOrParent() {
	r, ok := h.cursorRow()
	if !ok {
		return
	}
	st := &h.model.UniqueView
	if r.Value.IsDir && r.Expanded {
		st.Pane.SetCollapsed(r.ID, true)
		h.syncRows(r.ID)
		return
	}
	for i := st.Pane.Selected - 1; i >= 0; i-- {
		if h.model.UniqueList[i].Depth < r.Depth {
			st.Pane.Selected = i
			return
		}
	}
}

// CollapseAll closes every directory.
func (h *Handler) CollapseAll() {
	st := &h.model.UniqueView
	st.Pane.Collapsed = ui.DedupCollapsedSet(ui.UniqueExpandableIDs(st))
	h.syncRows(h.topAncestorOfCursor())
}

// topAncestorOfCursor is the row the cursor belongs on once everything is closed.
func (h *Handler) topAncestorOfCursor() string {
	st, rows := &h.model.UniqueView, h.model.UniqueList
	for i := min(st.Pane.Selected, len(rows)-1); i >= 0; i-- {
		if rows[i].Depth == 0 {
			return rows[i].ID
		}
	}
	return ""
}

// ExpandAll opens every directory.
func (h *Handler) ExpandAll() {
	cursor := h.cursorID()
	h.model.UniqueView.Pane.Collapsed = nil
	h.syncRows(cursor)
}

// ToggleHidden shows or hides files under dot-prefixed path components. A file that gets hidden
// loses its mark, so an operation never includes a file the tree does not show.
func (h *Handler) ToggleHidden() {
	st := &h.model.UniqueView
	cursor := h.cursorID()
	st.HideHidden = !st.HideHidden
	visible := st.VisibleFiles()
	present := make(map[string]int64, len(visible))
	for _, f := range visible {
		present[f.Path] = f.Size
	}
	h.keepMarksOf(present)
	h.rebuild(cursor)
	word := "shown"
	if st.HideHidden {
		word = "hidden"
	}
	h.host.SetTransientMessage("Hidden files "+word, ui.MessageUrgencyInfo)
}

// SelectToggleAndAdvance marks or unmarks the cursor file and moves down; on a directory it does
// so for every file below and moves past the directory.
func (h *Handler) SelectToggleAndAdvance() {
	r, ok := h.cursorRow()
	if !ok {
		return
	}
	st := &h.model.UniqueView
	if !r.Value.IsDir {
		st.SetMark(ui.UniqueFile{Path: r.Value.Path, Size: r.Value.Size}, !st.Marked[r.Value.Path])
		h.MoveSelection(1)
		return
	}
	files := st.FilesUnder(r.Value.Path)
	on := slices.ContainsFunc(files, func(f ui.UniqueFile) bool { return !st.Marked[f.Path] })
	for _, f := range files {
		st.SetMark(f, on)
	}
	rows := h.model.UniqueList
	i := st.Pane.Selected + 1
	for i < len(rows) && rows[i].Depth > r.Depth {
		i++
	}
	st.Pane.Selected = min(i, len(rows)-1)
}

// SelectRange is panel.select-range over the file rows: the first press anchors the cursor's file
// and moves down, the next marks every file row between anchor and cursor. Directory rows are
// skipped; the range covers only rows on screen, so collapsed directories are not included.
func (h *Handler) SelectRange() (res primitive.RangeResult, added int) {
	st, rows := &h.model.UniqueView, h.model.UniqueList
	r, ok := h.cursorRow()
	if st.FocusPanel || !ok || r.Value.IsDir {
		return primitive.RangeNone, 0
	}
	ai := primitive.RangeAnchorIndex(len(rows), st.RangeAnchor, func(i int) string { return rows[i].Value.Path })
	if ai < 0 {
		st.RangeAnchor = r.Value.Path
		h.MoveSelection(1)
		return primitive.RangeStarted, 0
	}
	for i := min(ai, st.Pane.Selected); i <= max(ai, st.Pane.Selected); i++ {
		d := rows[i].Value
		if d.IsDir {
			continue
		}
		if !st.Marked[d.Path] {
			added++
		}
		st.SetMark(ui.UniqueFile{Path: d.Path, Size: d.Size}, true)
	}
	st.RangeAnchor = ""
	return primitive.RangeDone, added
}

// ClearMarks drops every mark.
func (h *Handler) ClearMarks() {
	st := &h.model.UniqueView
	st.Marked, st.MarkedBytes, st.RangeAnchor = nil, 0, ""
}

// Sources are the files an operation acts on: the marked files, else the cursor file, else the
// files below the cursor directory.
func (h *Handler) Sources() []ui.UniqueFile {
	st := &h.model.UniqueView
	if marked := st.MarkedFiles(); len(marked) > 0 {
		return marked
	}
	r, ok := h.cursorRow()
	switch {
	case !ok:
		return nil
	case r.Value.IsDir:
		return st.FilesUnder(r.Value.Path)
	}
	return []ui.UniqueFile{{Path: r.Value.Path, Size: r.Value.Size, Unverified: r.Value.Unverified > 0}}
}

// Target is the cursor row's path.
func (h *Handler) Target() (path string, isDir, ok bool) {
	r, ok := h.cursorRow()
	return r.Value.Path, r.Value.IsDir, ok
}

// OpenableFile is the cursor row's path when it is a file.
func (h *Handler) OpenableFile() (string, bool) {
	path, isDir, ok := h.Target()
	return path, ok && !isDir
}

// OpenInPanel shows path in panelID: a directory itself, a file in its directory.
func (h *Handler) OpenInPanel(panelID int, path string, isDir bool) {
	dir, name := path, ""
	if !isDir {
		dir, name = filepath.Dir(path), filepath.Base(path)
	}
	if err := h.host.NavigatePanelToPath(panelID, dir, name); err != nil {
		h.host.SetTransientMessage("Open: "+err.Error(), ui.MessageUrgencyWarn)
		return
	}
	host.ReportOpenedInPanel(h.host, filepath.Base(path), panelID)
}

// FilesQueued records an operation queued on paths: marks go, and with gone (a move, a delete)
// the files leave the list; the cursor stays on the nearest row left. A failed job's files come
// back with the next refresh. A copy keeps its files listed: whether the copy lands outside the
// scope, in a place the index covers, is what the next refresh answers.
func (h *Handler) FilesQueued(paths []string, gone bool) {
	st := &h.model.UniqueView
	h.ClearMarks()
	if !gone {
		return
	}
	drop := make(map[string]bool, len(paths))
	for _, p := range paths {
		drop[p] = true
	}
	st.Files = slices.DeleteFunc(slices.Clone(st.Files), func(f ui.UniqueFile) bool { return drop[f.Path] })
	h.rebuild(h.cursorID())
}

// BeginDelete takes the files of a confirmed delete. It queues them right away when that leaves no
// directory empty; otherwise it returns those directories, relative to Root, for the caller to ask
// about, and FinishDelete queues them with the answer.
func (h *Handler) BeginDelete(paths []string) (emptyDirs []string) {
	if dirs := h.emptyDirsAfterDelete(paths); len(dirs) > 0 {
		h.pendingDelete = paths
		return dirs
	}
	h.deleteQueued(paths, false)
	return nil
}

// FinishDelete queues the delete waiting on the empty-directories question.
func (h *Handler) FinishDelete(removeEmptyDirs bool) {
	paths := h.pendingDelete
	h.pendingDelete = nil
	if len(paths) > 0 {
		h.deleteQueued(paths, removeEmptyDirs)
	}
}

// CancelDelete drops the delete waiting on the empty-directories question: nothing is deleted and
// the marks stay.
func (h *Handler) CancelDelete() { h.pendingDelete = nil }

// emptyDirsAfterDelete dry-runs the cleanup a delete of paths would do.
func (h *Handler) emptyDirsAfterDelete(paths []string) []string {
	removed := make(map[string]bool, len(paths))
	seen := map[string]bool{}
	var roots []pathloc.Path
	for _, p := range paths {
		loc, err := pathloc.Parse(p)
		if err != nil {
			continue
		}
		removed[loc.String()] = true
		parent := loc.Parent()
		if key := parent.String(); !seen[key] {
			seen[key] = true
			roots = append(roots, parent)
		}
	}
	dirs, err := ops.PreviewEmptyDirsUnder(context.Background(), roots, removed)
	if err != nil {
		return nil
	}
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		path, err := d.FilePath()
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(h.model.UniqueView.Root, path)
		if err != nil {
			rel = path
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

func (h *Handler) deleteQueued(paths []string, removeEmptyDirs bool) {
	h.host.EnqueueDeleteJob(paths, removeEmptyDirs)
	h.host.SetTransientMessage(fmt.Sprintf("Delete queued (%d %s)", len(paths), plural(len(paths))), ui.MessageUrgencyInfo)
	h.FilesQueued(paths, true)
}

func plural(n int) string {
	if n == 1 {
		return "file"
	}
	return "files"
}

// HashUnverified queues a full hash of the files behind the Unverified flags; the list refreshes
// when it ends.
func (h *Handler) HashUnverified() {
	st := &h.model.UniqueView
	if len(st.ToHash) == 0 {
		h.host.SetTransientMessage("Nothing to hash: every listed file is settled", ui.MessageUrgencyInfo)
		return
	}
	root := st.Root
	task := h.index.HashFilesTask(slices.Clone(st.ToHash), func() { h.post(HashedPayload{Root: root}) })
	var sources []pathloc.Path
	for _, dir := range st.Scope {
		if loc, err := pathloc.Parse(dir); err == nil {
			sources = append(sources, loc)
		}
	}
	h.host.AddTaskJob(task, sources)
	h.host.SetTransientMessage(fmt.Sprintf("Hashing queued (%d %s)", len(st.ToHash), plural(len(st.ToHash))), ui.MessageUrgencyInfo)
}

// SwitchPane moves focus between the tree and the destination panel.
func (h *Handler) SwitchPane() {
	st := &h.model.UniqueView
	st.FocusPanel = !st.FocusPanel
	st.Pane.Filter = quickfilter.Filter{}
}

// FocusPanel gives the destination panel focus (a click).
func (h *Handler) FocusPanel() {
	st := &h.model.UniqueView
	st.FocusPanel = true
	st.Pane.Filter = quickfilter.Filter{}
}

// Type-to-jump filter over the tree's rows (the listFilterCtrl surface).

func rowNames(rows []ui.UniqueRow) []string {
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r.Value.Display
	}
	return names
}

func (h *Handler) filterOptions() quickfilter.Options {
	f := h.host.Config().Filter
	return quickfilter.OptionsFrom(f.CaseInsensitive, f.CycleMatches)
}

func (h *Handler) applyFilter(query string) {
	st := &h.model.UniqueView
	if cur, ok := st.Pane.Filter.Apply(query, rowNames(h.model.UniqueList), h.filterOptions()); ok {
		st.Pane.Selected = cur
	}
}

// Filter is the tree's quick filter.
func (h *Handler) Filter() quickfilter.Filter { return h.model.UniqueView.Pane.Filter }

// AppendFilterRune types r into the filter.
func (h *Handler) AppendFilterRune(r rune) {
	h.applyFilter(h.model.UniqueView.Pane.Filter.InsertRune(r))
}

// BackspaceFilter deletes the rune before the filter caret.
func (h *Handler) BackspaceFilter() {
	if next, changed := h.model.UniqueView.Pane.Filter.Backspace(); changed {
		h.applyFilter(next)
	}
}

// ClearFilter empties the query while the filter stays open.
func (h *Handler) ClearFilter() { h.applyFilter("") }

// CancelFilter closes the filter.
func (h *Handler) CancelFilter() { h.model.UniqueView.Pane.Filter = quickfilter.Filter{} }

// CycleFilterMatch steps the cursor through the matches, or moves plainly when nothing matches.
func (h *Handler) CycleFilterMatch(delta int) {
	st := &h.model.UniqueView
	if cur, ok := st.Pane.Filter.Cycle(st.Pane.Selected, delta, h.filterOptions()); ok {
		st.Pane.Selected = cur
		return
	}
	h.MoveSelection(delta)
}
