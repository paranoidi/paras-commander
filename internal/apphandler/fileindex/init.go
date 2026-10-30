package fileindex

import (
	"errors"

	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// ReadyPayload reports whether the index opened before an action that needs it runs; Path is
// the database path when it is missing.
type ReadyPayload struct {
	Err  error
	Path string
	then func()
}

// InitPayload reports the outcome of creating the database the user agreed to create.
type InitPayload struct {
	Path string
	Err  error
	then func()
}

// withIndex runs then on the event loop once the index is open, checked off the UI thread. A
// missing database asks whether to create it (then runs after it is created); any other open
// failure is reported instead of running then.
func (h *Handler) withIndex(then func()) {
	go func() {
		_, err := h.index.DB()
		var path string
		if errors.Is(err, fi.ErrNoIndex) {
			path, _ = h.index.DatabasePath()
		}
		h.post(ReadyPayload{Err: err, Path: path, then: then})
	}()
}

// HandleReady runs the waiting action, or asks to create the missing database.
func (h *Handler) HandleReady(p ReadyPayload) {
	switch {
	case p.Err == nil:
		p.then()
	case errors.Is(p.Err, fi.ErrNoIndex):
		h.promptInit(p.Path, p.then)
	default:
		h.host.SetErrorMessage(messageTitle, p.Err)
	}
}

// promptInit opens the "create the fileindex database?" prompt for path; then (may be nil) runs
// once the database is created. A prompt already open only takes the newer action. With another
// dialog open there is nowhere to ask, so a passive open failure (then nil) is reported instead.
func (h *Handler) promptInit(path string, then func()) {
	if fd := h.model.FileDialog; fd.Open && fd.DialogType == dialog.FileDialogIndexInit {
		if then != nil {
			h.pendingInit = then
		}
		return
	}
	if h.model.AnyModalOpen() {
		h.host.SetErrorMessage(messageTitle, errors.New("no fileindex database at "+path))
		return
	}
	h.pendingInit = then
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogIndexInit,
		Message:    path,
		Focus:      dlg.OK,
	}
}

// SubmitInit is the prompt's Yes: it closes the prompt and creates the database off the UI thread.
func (h *Handler) SubmitInit() {
	then := h.pendingInit
	h.pendingInit = nil
	h.model.FileDialog = dialog.FileDialogState{}
	go func() {
		path, err := h.index.Create()
		h.post(InitPayload{Path: path, Err: err, then: then})
	}()
}

// HandleInit reports the created database and runs the action that asked for it.
func (h *Handler) HandleInit(p InitPayload) {
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	h.host.SetTransientMessage("Created fileindex database at "+p.Path, ui.MessageUrgencySuccess)
	h.host.RefreshMetaColumns()
	if p.then != nil {
		p.then()
	}
}
