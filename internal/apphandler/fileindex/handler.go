// Package fileindex owns the UI side of the built-in fileindex feature (ADR 0024): the
// fileindex.* actions, the tag prompt, and the wake payloads that bring database results back
// to the event loop. All database and disk work runs on goroutines (internal/fileindex); this
// package never imports internal/app (ADR 0001).
package fileindex

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gdamore/tcell/v2"
	findctrl "github.com/paranoidi/paras-commander/internal/apphandler/find"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	"github.com/paranoidi/paras-commander/internal/config"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Host supplies the app services the fileindex handler cannot import from internal/app.
type Host interface {
	host.MessageHost
	host.LayoutHost
	host.PanelHost
	host.PanelNavigationHost

	// AddTaskJob queues a task job (scan, prune) working on sources.
	AddTaskJob(task *jobs.Task, sources []pathloc.Path)
	// TaskUnfinished reports whether a job running task exists and has not finished.
	TaskUnfinished(task *jobs.Task) bool
	// SyncJobsList refreshes the jobs list mirror (a task's results can change after its job ended).
	SyncJobsList()
	// RefreshMetaColumns re-runs both panels' meta columns, so the tags column shows an edit.
	RefreshMetaColumns()
	// PanelByID and PanelViewportRows resolve the panel the find dialog works on.
	PanelByID(panelID int) *panel.State
	PanelViewportRows(panelID int) int
	// Styles is the live theme (the find dialog's spec depends on its icons).
	Styles() theme.Theme
	// OpenGroupSelectDialog opens the select / unselect group dialog for the find-in-index results.
	OpenGroupSelectDialog(mode findctrl.GroupSelectMode)
	// PinTogglePath pins or unpins path and shows the status message (name: display basename).
	PinTogglePath(name, path string, isDir bool)
	// OpenFullscreenFilePreviewAt opens path in the fullscreen file viewer.
	OpenFullscreenFilePreviewAt(path string, isDir bool) error
	// EditFile runs the external editor on the local file path; errors become an "Edit" message.
	EditFile(path string)
	// ToggleFileIndexMenu opens or closes the fileindex menu strip (app-owned leader-menu chrome).
	ToggleFileIndexMenu()
	// ReloadPanel re-reads the panel's listing (the overlay is applied or dropped by it).
	ReloadPanel(panelID int)
	// OpenFilePicker opens the path picker over items, host file paths (the differences report);
	// confirming one shows it in the active panel (a fileindex:// path when indexPanel).
	OpenFilePicker(title string, items []dialog.PathPickerItem, indexPanel bool)
	// OpenCopiesView opens the Other copies view over the copies of the file called origin;
	// indexPanel: the lookup started from a fileindex:// panel.
	OpenCopiesView(origin string, indexPanel bool, copies []fi.CopyFile)
	// OpenDedupOutside opens the find-duplicates view over the files under the host dir dir that
	// have a copy elsewhere in the index.
	OpenDedupOutside(dir string)
	// OpenDedupIndexed opens the find-duplicates view over the indexed files under the host dir
	// dir, whatever kind of panel is active.
	OpenDedupIndexed(dir string)
	// OpenUniqueFiles opens the Find unique files view for the active panel.
	OpenUniqueFiles()
}

// Deps wires the fileindex handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model
	Index  *fi.Index
	Config config.Config
	// Keys is the global keymap; KeysFindDialog the [dialog.find] overlay the find-in-index
	// dialog shares with Find; KeysDialogInput the [dialog.input] overlay for the query.
	Keys            *keymap.Map
	KeysFindDialog  *keymap.Map
	KeysDialogInput *keymap.Map
}

// Handler runs the fileindex.* actions.
type Handler struct {
	host                            Host
	screen                          tcell.Screen
	model                           *ui.Model
	index                           *fi.Index
	config                          config.Config
	keys, keysFind, keysDialogInput *keymap.Map
	// findTimer is the debounce of the find-in-index dialog's pending query (UI goroutine only).
	findTimer *time.Timer
	// findHiddenForView is set while the find-in-index dialog is hidden behind the fullscreen
	// viewer it opened (find.view): IndexFindDialog.Open is false, a query in flight still lands,
	// and ReopenFindAfterPreviewClose shows the dialog again as it was.
	findHiddenForView bool
	// pendingInit is the action waiting on the open "create the database?" prompt (nil: none).
	pendingInit func()
	// initRunning is set while a confirmed create or upgrade runs (menu-bar spinner).
	initRunning bool
	// browseAny and browseStructure are the Match and View Browse by tags used last (session only;
	// All and Flat until it is used).
	browseAny, browseStructure bool
}

// New constructs the handler and routes the index's one-time open failure to HandleReady.
func New(deps Deps) *Handler {
	h := &Handler{
		host: deps.Host, screen: deps.Screen, model: deps.Model, index: deps.Index, config: deps.Config,
		keys: deps.Keys, keysFind: deps.KeysFindDialog, keysDialogInput: deps.KeysDialogInput,
	}
	if h.index != nil {
		h.index.OnOpenError = func(err error) {
			path, _ := h.index.DatabasePath()
			h.post(ReadyPayload{Err: err, Path: path})
		}
	}
	return h
}

// NavigatePayload asks the event loop to show Dir in PanelID with Select highlighted, once a
// goroutine has checked the target (Err set when it cannot be shown).
type NavigatePayload struct {
	PanelID int
	Dir     string
	Select  string
	Err     error
	// WithIndex turns the browse-with-fileindex overlay on for the panel before it navigates.
	WithIndex bool
	// Warn is shown instead of navigating (Dir is then empty).
	Warn string
}

// Dispatch runs a fileindex.* action; it reports false for any other action id.
func (h *Handler) Dispatch(actionID string) bool {
	switch actionID {
	case keymap.ActionFileIndexMenu:
		h.host.ToggleFileIndexMenu()
	case keymap.ActionFileIndexScan:
		h.WithIndex(h.Scan)
	case keymap.ActionFileIndexScanFast:
		h.WithIndex(h.ScanFast)
	case keymap.ActionFileIndexPrune:
		h.WithIndex(h.Prune)
	case keymap.ActionFileIndexTags:
		h.WithIndex(h.OpenTagDialog)
	case keymap.ActionFileIndexFind:
		if h.localOrIndex() {
			h.OpenFindDialog(h.model.FocusedPanelID())
		}
	case keymap.ActionFileIndexFindUnique:
		if h.localOrIndex() {
			h.WithIndex(h.host.OpenUniqueFiles)
		}
	case keymap.ActionFileIndexBrowseWith:
		h.WithIndex(h.BrowseWith)
	case keymap.ActionFileIndexBrowse:
		h.WithIndex(h.Browse)
	case keymap.ActionFileIndexBrowseTags:
		h.WithIndex(h.OpenBrowseTags)
	case keymap.ActionFileIndexTagViewMode:
		h.ToggleTagViewMode()
	case keymap.ActionFileIndexReveal:
		h.Reveal()
	case keymap.ActionFileIndexCopies:
		h.Copies()
	case keymap.ActionFileIndexReportDifferences:
		h.WithIndex(h.ReportDifferences)
	case keymap.ActionFileIndexCopiesOutside:
		h.WithIndex(h.CopiesOutside)
	case keymap.ActionFileIndexFindDuplicates:
		h.WithIndex(h.FindDuplicates)
	default:
		return false
	}
	return true
}

// HandleWake applies one of this handler's wake payloads on the event loop; it reports false
// for any other payload.
func (h *Handler) HandleWake(data any) bool {
	switch p := data.(type) {
	case ReadyPayload:
		h.HandleReady(p)
	case InitPayload:
		h.HandleInit(p)
	case TagsLoadedPayload:
		h.HandleTagsLoaded(p)
	case TagsPayload:
		h.HandleTags(p)
	case BrowseTagsLoadedPayload:
		h.HandleBrowseTagsLoaded(p)
	case NavigatePayload:
		h.HandleNavigate(p)
	case FindPayload:
		h.HandleFind(p)
	case CopiesPayload:
		h.HandleCopies(p)
	case DifferencesPayload:
		h.HandleDifferences(p)
	case DifferencesQueuedPayload:
		h.HandleDifferencesQueued(p)
	default:
		return false
	}
	return true
}

// post hands payload to the event loop. The app's screen never drops a posted event.
func (h *Handler) post(payload any) {
	if h.screen == nil {
		return
	}
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(payload))
}

const messageTitle = "fileindex"

// targets returns the active panel's selection, else its cursor entry, else its directory; nil
// when that would be the list of a flat tag view (see refuseTagList).
func (h *Handler) targets() []string {
	if src := h.host.FocusedPanelSources(); len(src) > 0 {
		return src
	}
	if h.refuseTagList() {
		return nil
	}
	return []string{h.host.FocusedPanel().PathString()}
}

// refuseTagList says so and reports true when the active panel shows the list of a flat tag view,
// whose directory ("/") is no real target: the guard of every action that falls back to the panel
// directory.
func (h *Handler) refuseTagList() bool {
	return host.RefuseTagList(h.host, h.host.FocusedPanel().Path)
}

// localOrIndex reports whether the active panel's scheme has index paths (local or fileindex://);
// otherwise it says why not.
func (h *Handler) localOrIndex() bool {
	if h.host.FocusedPanel().Path.Scheme() == pathloc.SchemeSFTP {
		h.host.SetTransientMessage("fileindex covers local paths only", ui.MessageUrgencyWarn)
		return false
	}
	return true
}

// errNoHostPath is the refusal for a panel path with no real path on disk (SFTP).
var errNoHostPath = errors.New("fileindex covers local paths only")

func (h *Handler) queueTask(title string, task *jobs.Task, paths []string) {
	h.addTask(task, paths)
	h.host.SetTransientMessage(title+" queued", ui.MessageUrgencyInfo)
}

// addTask queues task as a job working on paths, without a toast.
func (h *Handler) addTask(task *jobs.Task, paths []string) {
	sources := make([]pathloc.Path, 0, len(paths))
	for _, p := range paths {
		if loc, err := pathloc.Parse(p); err == nil {
			sources = append(sources, loc)
		}
	}
	h.host.AddTaskJob(task, sources)
}

// Scan queues a scan of the selection, else the panel directory (the
// real paths, also from a fileindex:// panel), and clears the selection.
func (h *Handler) Scan() { h.queueScan("fileindex scan", h.index.ScanTask) }

// ScanFast is Scan hashing only the sample (first, middle and last 8 KB) of larger files.
func (h *Handler) ScanFast() { h.queueScan("fileindex fast scan", h.index.ScanFastTask) }

func (h *Handler) queueScan(title string, task func([]string) *jobs.Task) {
	if !h.localOrIndex() {
		return
	}
	// Explicit selections only; never the cursor entry.
	var targets []string
	if len(h.host.FocusedPanel().SelectedPaths) > 0 {
		targets = h.targets()
	} else if !h.refuseTagList() {
		targets = []string{h.host.FocusedPanel().PathString()}
	}
	if len(targets) == 0 {
		return
	}
	var paths []string
	for _, t := range targets {
		p, ok := pathloc.HostOf(t)
		if !ok {
			h.host.SetErrorMessage(messageTitle, errNoHostPath)
			return
		}
		paths = append(paths, p)
	}
	h.queueTask(title, task(paths), paths)
	h.host.FocusedPanel().ClearSelection()
}

// ScanAfterCommand queues a scan of dir (user-menu fileindex_scan, the command already ran). It
// shows no toast and omits the job's started and completed ones: the command's own notification is
// the one to keep on screen, and the job list shows the scan. A failed scan still reports.
func (h *Handler) ScanAfterCommand(dir pathloc.Path) {
	if host.RefuseTagList(h.host, dir) { // the list's "/" is no directory to scan (an index panel is refused before the command runs)
		return
	}
	p, ok := pathloc.HostOf(dir.String())
	if !ok {
		h.host.SetErrorMessage(messageTitle, errNoHostPath)
		return
	}
	h.WithIndex(func() {
		paths := []string{p}
		task := h.index.ScanTask(paths)
		task.ReportsOwnOutcome = true
		h.addTask(task, paths)
	})
}

// Prune queues a prune of the active panel's directory.
func (h *Handler) Prune() {
	if !h.localOrIndex() || h.refuseTagList() {
		return
	}
	dir, ok := pathloc.HostOf(h.host.FocusedPanel().PathString())
	if !ok {
		h.host.SetErrorMessage(messageTitle, errNoHostPath)
		return
	}
	h.queueTask("fileindex prune", h.index.PruneTask(dir), []string{dir})
}

// BrowseWith toggles "browse with fileindex" on the active panel: the real directory with the
// index merged in (state icons, ghost rows for index-only entries). On a fileindex:// panel it
// goes to the real location with the overlay on.
func (h *Handler) BrowseWith() {
	if !h.localOrIndex() {
		return
	}
	p := h.host.FocusedPanel()
	if p.Path.Scheme() == pathloc.SchemeFileIndex {
		h.reveal(true)
		return
	}
	h.setBrowseWith(!p.WithIndex)
}

// setBrowseWith turns the overlay on or off for the active panel and reloads it.
func (h *Handler) setBrowseWith(on bool) {
	h.host.FocusedPanel().WithIndex = on
	h.host.ReloadPanel(h.model.FocusedPanelID())
	msg := "Browse with fileindex off"
	if on {
		msg = "Browse with fileindex on"
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
}

// Browse shows the active panel's directory in fileindex://, keeping the cursor entry; on a
// fileindex:// panel it goes back to the real location, and from a tag view to the plain index
// (browseFromTagView).
func (h *Handler) Browse() {
	p := h.host.FocusedPanel()
	if _, ok := p.Path.TagQuery(); ok {
		h.browseFromTagView()
		return
	}
	if p.Path.Scheme() == pathloc.SchemeFileIndex {
		h.Reveal() // toggle back to the real location
		return
	}
	if !h.localOrIndex() {
		return
	}
	panelID, dir := h.model.FocusedPanelID(), p.PathString()
	var sel string
	if e, ok := p.CurrentEntry(); ok {
		sel = e.Name
	}
	go func() {
		loc, err := fi.IndexLocation(dir)
		if err == nil {
			_, err = fsbackend.Default().Stat(context.Background(), loc)
			if errors.Is(err, fs.ErrNotExist) {
				err = fmt.Errorf("%s is not in the fileindex", dir)
			}
		}
		h.post(NavigatePayload{PanelID: panelID, Dir: loc.String(), Select: sel, Err: err})
	}()
}

// browseFromTagView shows the plain index at the cursor entry's directory with the entry selected,
// or at the panel's own host directory with no entry. The entry comes from the index, so nothing
// needs checking off the UI thread.
func (h *Handler) browseFromTagView() {
	p := h.host.FocusedPanel()
	_, dir, _ := pathloc.CutIndexPrefix(p.Path.String())
	sel := ""
	if e, ok := p.CurrentEntry(); ok && e.Name != ".." {
		_, entryHost, _ := pathloc.CutIndexPrefix(e.Path)
		dir, sel = pathloc.Dir(entryHost), filepath.Base(entryHost)
	}
	loc, err := pathloc.FileIndex(dir)
	if err == nil {
		err = h.host.NavigatePanelToPath(h.model.FocusedPanelID(), loc.String(), sel)
	}
	if err != nil {
		h.host.SetErrorMessage(messageTitle, err)
	}
}

// Reveal shows the cursor entry of a fileindex:// panel at its real location (its parent
// directory on disk, the entry highlighted), or the panel's own directory with no entry. With
// browse with fileindex on, the panel already is the real directory, so it turns the overlay off.
func (h *Handler) Reveal() {
	if p := h.host.FocusedPanel(); p.WithIndex && p.Path.Scheme() != pathloc.SchemeFileIndex {
		h.setBrowseWith(false)
		return
	}
	h.reveal(false)
}

// reveal is Reveal; withIndex also turns the browse-with-fileindex overlay on for the target.
func (h *Handler) reveal(withIndex bool) {
	p := h.host.FocusedPanel()
	if p.Path.Scheme() != pathloc.SchemeFileIndex {
		h.host.SetTransientMessage("Browse filesystem works on index panels", ui.MessageUrgencyWarn)
		return
	}
	panelID := h.model.FocusedPanelID()
	target := p.PathString()
	if e, ok := p.CurrentEntry(); ok {
		target = e.Path
	} else if h.refuseTagList() { // an empty list: its "/" is not a place to go
		return
	}
	host, ok := pathloc.HostOf(target)
	if !ok {
		h.host.SetErrorMessage(messageTitle, errNoHostPath)
		return
	}
	go func() {
		dir, sel := pathloc.Dir(host), filepath.Base(host)
		if host == "/" {
			dir, sel = host, ""
		}
		_, err := os.Stat(host)
		if errors.Is(err, fs.ErrNotExist) {
			err = fmt.Errorf("%s is not on disk", host)
		}
		h.post(NavigatePayload{PanelID: panelID, Dir: dir, Select: sel, Err: err, WithIndex: withIndex})
	}()
}

// HandleNavigate applies a Browse, Reveal or Browse by tags once its target was checked.
func (h *Handler) HandleNavigate(p NavigatePayload) {
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	if p.Warn != "" {
		h.host.SetTransientMessage(p.Warn, ui.MessageUrgencyWarn)
		return
	}
	if p.WithIndex {
		h.host.PanelByID(p.PanelID).WithIndex = true
	}
	if err := h.host.NavigatePanelToPath(p.PanelID, p.Dir, p.Select); err != nil {
		h.host.SetErrorMessage(messageTitle, err)
	}
}

// SetConfig replaces the handler's configuration snapshot (configuration reload). Called on the
// UI goroutine; work already started keeps the values it read.
func (h *Handler) SetConfig(cfg config.Config) {
	h.config = cfg
}
