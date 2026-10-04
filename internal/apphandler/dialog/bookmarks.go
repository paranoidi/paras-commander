package dialog

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/bookmarks"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// bookmarkIOKind identifies which marks-file operation a BookmarkIOPayload completes.
type bookmarkIOKind int

const (
	bookmarkIOLoad bookmarkIOKind = iota + 1
	bookmarkIOAppend
	bookmarkIORemove
)

// BookmarkIOPayload carries a background fzf-marks/GTK read or atomic write back to the
// event loop. Gen identifies the start that produced it; ApplyBookmarkIO drops a stale
// result so a superseded open/add/delete cannot mutate UI state.
type BookmarkIOPayload struct {
	Gen         uint64
	Err         error
	Items       []dialog.PathPickerItem
	Name        string
	MarksPath   string
	Label       string
	RemoveIndex int
	kind        bookmarkIOKind
}

// bookmarkIOStall, when set, runs on the worker before marks-file I/O so tests can hold a
// delayed filesystem. bookmarkIOSkipInline disables the test-only inline apply so the
// completion is left on the event queue.
var (
	bookmarkIOStall      func()
	bookmarkIOSkipInline atomic.Bool
)

func bookmarkIOInline() bool {
	return testing.Testing() && !bookmarkIOSkipInline.Load()
}

func stallBookmarkFileIO() {
	if bookmarkIOStall != nil {
		bookmarkIOStall()
	}
}

func (h *Handler) postBookmarkIO(p BookmarkIOPayload) {
	if h.screen == nil {
		return
	}
	_ = h.screen.PostEvent(tcell.NewEventInterrupt(p))
}

// ApplyBookmarkIO applies a background bookmark load/append/remove unless a newer
// operation has been started (or the picker closed) in the meantime.
func (h *Handler) ApplyBookmarkIO(p BookmarkIOPayload) {
	switch p.kind {
	case bookmarkIOLoad:
		h.applyBookmarkLoad(p)
	case bookmarkIOAppend:
		h.applyBookmarkAppend(p)
	case bookmarkIORemove:
		h.applyBookmarkRemove(p)
	}
}

func (h *Handler) applyBookmarkLoad(p BookmarkIOPayload) {
	if !h.model.PathPicker.Open || p.Gen != h.pathPickerMissingGen {
		return
	}
	st := &h.model.PathPicker
	if p.Err != nil {
		if len(st.Items) == 0 {
			h.ClosePathPicker()
		}
		h.host.SetErrorMessage("Bookmarks", p.Err)
		return
	}
	merged := p.Items
	bookmarked := make(map[string]struct{}, len(p.Items))
	for _, it := range p.Items {
		bookmarked[it.Path] = struct{}{}
	}
	for _, it := range st.Items {
		if _, ok := bookmarked[it.Path]; !ok {
			merged = append(merged, it)
		}
	}
	if len(merged) == 0 && st.Purpose != dialog.PathPickerPurposeNavigate {
		msg := "No bookmarks"
		if st.Title == "All paths" {
			msg = "No paths"
		}
		h.ClosePathPicker()
		h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
		return
	}
	st.Items = merged
	h.SyncPathPickerRanks()
	h.startPathPickerMissingScan()
}

func (h *Handler) applyBookmarkAppend(p BookmarkIOPayload) {
	if p.Gen != h.remoteFileOpGen {
		return
	}
	if p.Err != nil {
		h.host.SetErrorMessage("Add bookmark", p.Err)
		return
	}
	h.host.SetTransientMessage(fmt.Sprintf("Bookmark added: %s → %s", p.Name, p.MarksPath), ui.MessageUrgencyInfo)
}

func (h *Handler) applyBookmarkRemove(p BookmarkIOPayload) {
	if p.Gen != h.remoteFileOpGen {
		return
	}
	if p.Err != nil {
		h.host.SetErrorMessage("Delete bookmark", p.Err)
		return
	}
	st := &h.model.PathPicker
	if st.Open && p.RemoveIndex >= 0 && p.RemoveIndex < len(st.Items) {
		st.Items = append(st.Items[:p.RemoveIndex], st.Items[p.RemoveIndex+1:]...)
		h.SyncPathPickerRanks()
	}
	h.host.SetTransientMessage(fmt.Sprintf("Bookmark removed: %s", p.Label), ui.MessageUrgencyInfo)
}

// OpenBookmarkDialog opens the fuzzy bookmarks path picker (fzf-marks entries merged with
// GNOME/GTK bookmarks) for navigating the active panel. Marks-file I/O runs on a worker;
// ApplyBookmarkIO fills the list when the read completes.
func (h *Handler) OpenBookmarkDialog() {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	if h.host.InQuickFilterUI() {
		h.host.ActivePanel().CancelFilter(h.host.ActiveViewportRows())
	}
	h.openBookmarkPathPicker(dialog.PathPickerPurposeNavigate, 0)
}

// startBookmarkListLoad reads fzf-marks/GTK bookmarks on a worker and posts a
// BookmarkIOPayload so ApplyBookmarkIO can fill the open picker on the event loop.
func (h *Handler) startBookmarkListLoad(gen uint64) {
	result := make(chan BookmarkIOPayload, 1)
	go func() {
		stallBookmarkFileIO()
		items, err := h.PathPickerItemsBookmarks()
		p := BookmarkIOPayload{Gen: gen, Err: err, Items: items, kind: bookmarkIOLoad}
		if bookmarkIOInline() {
			result <- p
		} else {
			h.postBookmarkIO(p)
		}
	}()
	if bookmarkIOInline() {
		h.ApplyBookmarkIO(<-result)
	}
}

// OpenAddBookmarkDialog presents the centered dialog to append a new fzf-marks entry
// for the active panel directory. Refuses while jobs view is active and cancels any
// open quick filter (mirroring OpenBookmarkDialog).
func (h *Handler) OpenAddBookmarkDialog() {
	if ui.IsAuxiliaryView(h.model.ViewMode) {
		return
	}
	if h.host.InQuickFilterUI() {
		h.host.ActivePanel().CancelFilter(h.host.ActiveViewportRows())
	}
	path := h.host.ActivePanel().PathString()
	if strings.TrimSpace(path) == "" {
		h.host.SetErrorMessage("Add bookmark", fmt.Errorf("no active panel path"))
		return
	}
	defaultName := DefaultBookmarkName(path)
	cursor := len([]rune(defaultName))
	pending := defaultName != ""
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogAddBookmark,
		Fields: []dialog.FileDialogField{
			{
				Label:          "Name",
				Value:          defaultName,
				Prefill:        defaultName,
				Cursor:         cursor,
				PrefillPending: pending,
			},
		},
		Message: path,
	}
}

// DefaultBookmarkName returns a sensible suggested mark name for path.
// Uses the basename, falling back to "root" for "/" or empty results.
func DefaultBookmarkName(path string) string {
	base := filepath.Base(filepath.Clean(path))
	switch base {
	case "", ".", string(filepath.Separator):
		return "root"
	}
	return base
}

// addBookmarkDialogInputField returns the name field for Add bookmark, including when
// keyboard focus is on OK/Cancel (focusedField returns nil in that case).
func (h *Handler) addBookmarkDialogInputField() *dialog.FileDialogField {
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogAddBookmark || len(d.Fields) != 1 {
		return nil
	}
	if d.FocusedField >= 0 && d.FocusedField < len(d.Fields) {
		return &d.Fields[d.FocusedField]
	}
	return &d.Fields[0]
}

// ExecuteAddBookmark validates the input, resolves the marks file, and appends
// a new mark line on a worker. The dialog closes immediately; ApplyBookmarkIO
// shows the transient banner when the atomic write completes.
func (h *Handler) ExecuteAddBookmark() {
	field := h.addBookmarkDialogInputField()
	if field == nil {
		h.CloseFileDialog()
		return
	}
	name := strings.TrimSpace(field.Value)
	path := strings.TrimSpace(h.model.FileDialog.Message)
	if name == "" {
		h.host.SetErrorMessage("Add bookmark", fmt.Errorf("name is required"))
		h.CloseFileDialog()
		return
	}
	if path == "" {
		h.host.SetErrorMessage("Add bookmark", fmt.Errorf("missing target path"))
		h.CloseFileDialog()
		return
	}
	marksPath, err := bookmarks.ResolveFile(h.host.Config().Bookmarks.File, h.model.UserHomeDir)
	if err != nil {
		h.host.SetErrorMessage("Add bookmark", err)
		h.CloseFileDialog()
		return
	}
	h.CloseFileDialog()
	gen := h.nextRemoteFileOpGen()
	result := make(chan BookmarkIOPayload, 1)
	go func() {
		stallBookmarkFileIO()
		err := bookmarks.Append(marksPath, bookmarks.Mark{Name: name, Path: path})
		p := BookmarkIOPayload{Gen: gen, Err: err, Name: name, MarksPath: marksPath, kind: bookmarkIOAppend}
		if bookmarkIOInline() {
			result <- p
		} else {
			h.postBookmarkIO(p)
		}
	}()
	if bookmarkIOInline() {
		h.ApplyBookmarkIO(<-result)
	}
}
