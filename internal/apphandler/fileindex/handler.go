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

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/apphandler/host"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/fsbackend"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// Host supplies the app services the fileindex handler cannot import from internal/app.
type Host interface {
	host.MessageHost
	host.PanelHost
	host.PanelNavigationHost

	// AddTaskJob queues a task job (scan, prune) working on sources.
	AddTaskJob(task *jobs.Task, sources []pathloc.Path)
	// RefreshMetaColumns re-runs both panels' meta columns, so the tags column shows an edit.
	RefreshMetaColumns()
}

// Deps wires the fileindex handler at app construction.
type Deps struct {
	Host   Host
	Screen tcell.Screen
	Model  *ui.Model
	Index  *fi.Index
}

// Handler runs the fileindex.* actions.
type Handler struct {
	host   Host
	screen tcell.Screen
	model  *ui.Model
	index  *fi.Index
}

// New constructs the handler and routes the index's one-time open failure to HandleOpenError.
func New(deps Deps) *Handler {
	h := &Handler{host: deps.Host, screen: deps.Screen, model: deps.Model, index: deps.Index}
	if h.index != nil {
		h.index.OnOpenError = func(err error) { h.post(OpenErrorPayload{Err: err}) }
	}
	return h
}

// OpenErrorPayload carries the index's first open failure (no index, wrong schema).
type OpenErrorPayload struct{ Err error }

// TagsPayload carries the outcome of a tag edit.
type TagsPayload struct {
	Remove bool
	Res    fi.TagEdit
	Err    error
}

// NavigatePayload asks the event loop to show Dir in PanelID with Select highlighted, once a
// goroutine has checked the target (Err set when it cannot be shown).
type NavigatePayload struct {
	PanelID int
	Dir     string
	Select  string
	Err     error
}

// Dispatch runs a fileindex.* action; it reports false for any other action id.
func (h *Handler) Dispatch(actionID string) bool {
	switch actionID {
	case keymap.ActionFileIndexScan:
		h.Scan()
	case keymap.ActionFileIndexPrune:
		h.Prune()
	case keymap.ActionFileIndexTagAdd:
		h.OpenTagDialog(false)
	case keymap.ActionFileIndexTagRemove:
		h.OpenTagDialog(true)
	case keymap.ActionFileIndexOpen:
		h.Open()
	case keymap.ActionFileIndexReveal:
		h.Reveal()
	default:
		return false
	}
	return true
}

// HandleWake applies one of this handler's wake payloads on the event loop; it reports false
// for any other payload.
func (h *Handler) HandleWake(data any) bool {
	switch p := data.(type) {
	case OpenErrorPayload:
		h.HandleOpenError(p)
	case TagsPayload:
		h.HandleTags(p)
	case NavigatePayload:
		h.HandleNavigate(p)
	default:
		return false
	}
	return true
}

func (h *Handler) post(payload any) {
	if h.screen != nil {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(payload))
	}
}

const messageTitle = "fileindex"

// HandleOpenError reports the index open failure once, to the Messages log.
func (h *Handler) HandleOpenError(p OpenErrorPayload) {
	h.host.SetErrorMessage(messageTitle, p.Err)
}

// targets returns the active panel's selection, else its cursor entry, else its directory.
func (h *Handler) targets() []string {
	if src := h.host.ActivePanelSources(); len(src) > 0 {
		return src
	}
	return []string{h.host.ActivePanel().PathString()}
}

// localOrIndex reports whether the active panel's scheme has index paths (local or fileindex://);
// otherwise it says why not.
func (h *Handler) localOrIndex() bool {
	if h.host.ActivePanel().Path.Scheme() == pathloc.SchemeSFTP {
		h.host.SetTransientMessage("fileindex covers local paths only", ui.MessageUrgencyWarn)
		return false
	}
	return true
}

// hostPath is the disk path a panel path names: itself on a local panel, the indexed host path
// on a fileindex:// panel. No I/O (symlinks are resolved by the task that uses it).
func hostPath(raw string) (string, error) {
	loc, err := pathloc.Parse(raw)
	if err != nil {
		return "", err
	}
	if loc.Scheme() == pathloc.SchemeFileIndex {
		return pathloc.IndexHostPath(loc)
	}
	return loc.FilePath()
}

func (h *Handler) queueTask(title string, task *jobs.Task, paths []string) {
	sources := make([]pathloc.Path, 0, len(paths))
	for _, p := range paths {
		if loc, err := pathloc.Parse(p); err == nil {
			sources = append(sources, loc)
		}
	}
	h.host.AddTaskJob(task, sources)
	h.host.SetTransientMessage(title+" queued", ui.MessageUrgencyInfo)
}

// Scan queues a scan of the selection, else the cursor entry, else the panel directory (the
// real paths, also from a fileindex:// panel).
func (h *Handler) Scan() {
	if !h.localOrIndex() {
		return
	}
	var paths []string
	for _, t := range h.targets() {
		p, err := hostPath(t)
		if err != nil {
			h.host.SetErrorMessage(messageTitle, err)
			return
		}
		paths = append(paths, p)
	}
	h.queueTask("fileindex scan", h.index.ScanTask(paths), paths)
}

// Prune queues a prune of the active panel's directory.
func (h *Handler) Prune() {
	if !h.localOrIndex() {
		return
	}
	dir, err := hostPath(h.host.ActivePanel().PathString())
	if err != nil {
		h.host.SetErrorMessage(messageTitle, err)
		return
	}
	h.queueTask("fileindex prune", h.index.PruneTask(dir), []string{dir})
}

// OpenTagDialog opens the tag prompt for the selection, else the cursor entry, else the panel
// directory; remove picks Remove tags over Add tags.
func (h *Handler) OpenTagDialog(remove bool) {
	if !h.localOrIndex() {
		return
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:             true,
		DialogType:       dialog.FileDialogIndexTags,
		Fields:           []dialog.FileDialogField{{Label: "Tags (comma or space separated)"}},
		IndexTagsRemove:  remove,
		IndexTagsTargets: h.targets(),
	}
}

// SubmitTags is the tag prompt's OK: it closes the prompt and edits the tags off the UI thread.
// An empty tag list keeps the prompt open.
func (h *Handler) SubmitTags() {
	st := h.model.FileDialog
	var raw string
	if len(st.Fields) > 0 {
		raw = st.Fields[0].Value
	}
	tags := fi.ParseTags(raw)
	if len(tags) == 0 {
		h.host.SetTransientMessage("Type at least one tag", ui.MessageUrgencyWarn)
		return
	}
	h.model.FileDialog = dialog.FileDialogState{}
	targets, remove, recursive := st.IndexTagsTargets, st.IndexTagsRemove, st.IndexTagsRecursive
	go func() {
		res, err := h.index.EditTags(targets, tags, !remove, recursive)
		h.post(TagsPayload{Remove: remove, Res: res, Err: err})
	}()
}

// HandleTags reports a finished tag edit and refreshes the tags column.
func (h *Handler) HandleTags(p TagsPayload) {
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	verb := "Added"
	if p.Remove {
		verb = "Removed"
	}
	msg := fmt.Sprintf("%s %d %s on %d %s", verb, p.Res.Changed, jobbridge.Plural(p.Res.Changed, "tag", "tags"),
		p.Res.Targeted, jobbridge.Plural(p.Res.Targeted, "entry", "entries"))
	urgency := ui.MessageUrgencySuccess
	if n := len(p.Res.Unknown); n > 0 {
		msg += fmt.Sprintf("; %d not in the index", n)
		urgency = ui.MessageUrgencyWarn
	}
	h.host.SetTransientMessage(msg, urgency)
	h.host.RefreshMetaColumns()
}

// Open shows the active panel's directory in fileindex://, keeping the cursor entry.
func (h *Handler) Open() {
	p := h.host.ActivePanel()
	switch p.Path.Scheme() {
	case pathloc.SchemeFileIndex:
		h.host.SetTransientMessage("Already browsing the fileindex", ui.MessageUrgencyInfo)
		return
	case pathloc.SchemeSFTP:
		h.host.SetTransientMessage("fileindex covers local paths only", ui.MessageUrgencyWarn)
		return
	}
	panelID, dir := h.model.ActivePanel, p.PathString()
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

// Reveal shows the cursor entry of a fileindex:// panel at its real location (its parent
// directory on disk, the entry highlighted), or the panel's own directory with no entry.
func (h *Handler) Reveal() {
	p := h.host.ActivePanel()
	if p.Path.Scheme() != pathloc.SchemeFileIndex {
		h.host.SetTransientMessage("Reveal works on fileindex:// panels", ui.MessageUrgencyWarn)
		return
	}
	panelID := h.model.ActivePanel
	target := p.PathString()
	if e, ok := p.CurrentEntry(); ok {
		target = e.Path
	}
	host, err := hostPath(target)
	if err != nil {
		h.host.SetErrorMessage(messageTitle, err)
		return
	}
	go func() {
		dir, sel := filepath.Dir(host), filepath.Base(host)
		if host == "/" {
			dir, sel = host, ""
		}
		_, err := os.Stat(host)
		if errors.Is(err, fs.ErrNotExist) {
			err = fmt.Errorf("%s is not on disk", host)
		}
		h.post(NavigatePayload{PanelID: panelID, Dir: dir, Select: sel, Err: err})
	}()
}

// HandleNavigate applies an Open or Reveal once its target was checked.
func (h *Handler) HandleNavigate(p NavigatePayload) {
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	if err := h.host.NavigatePanelToPath(p.PanelID, p.Dir, p.Select); err != nil {
		h.host.SetErrorMessage(messageTitle, err)
	}
}
