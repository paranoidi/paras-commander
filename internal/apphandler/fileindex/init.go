package fileindex

import (
	"errors"

	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// ReadyPayload reports whether the index opened before an action that needs it runs; Path is
// the database path when it is missing or needs an upgrade.
type ReadyPayload struct {
	Err  error
	Path string
	then func()
}

// InitPayload reports the outcome of creating (or, with Upgrade, upgrading) the database the
// user agreed to.
type InitPayload struct {
	Path    string
	Upgrade bool
	Err     error
	then    func()
}

// withIndex runs then on the event loop once the index is open, checked off the UI thread. A
// missing database asks whether to create it, an outdated one whether to upgrade it (then runs
// once that is done); any other open failure is reported instead of running then.
func (h *Handler) withIndex(then func()) {
	go func() {
		_, err := h.index.DB()
		var path string
		if errors.Is(err, fi.ErrNoIndex) || fi.NeedsUpgrade(err) {
			path, _ = h.index.DatabasePath()
		}
		h.post(ReadyPayload{Err: err, Path: path, then: then})
	}()
}

// HandleReady runs the waiting action, or asks to create or upgrade the database. The index's
// own first open failure (from whatever touched it first) arrives here with no action waiting.
func (h *Handler) HandleReady(p ReadyPayload) {
	switch {
	case p.Err == nil:
		if p.then != nil {
			p.then()
		}
	case errors.Is(p.Err, fi.ErrNoIndex):
		h.promptInit(p.Path, false, p.then)
	case fi.NeedsUpgrade(p.Err):
		h.promptInit(p.Path, true, p.then)
	default:
		h.host.SetErrorMessage(messageTitle, p.Err)
	}
}

// promptInit opens the "create the fileindex database?" prompt for path, or with upgrade the
// "upgrade it?" one; then (may be nil) runs once that is done. A prompt already open only takes
// the newer action. With another dialog open there is nowhere to ask, so the problem is reported
// instead.
func (h *Handler) promptInit(path string, upgrade bool, then func()) {
	if fd := h.model.FileDialog; fd.Open && fd.DialogType == dialog.FileDialogIndexInit {
		if then != nil {
			h.pendingInit = then
		}
		return
	}
	if h.model.AnyModalOpen() {
		problem := "no fileindex database at "
		if upgrade {
			problem = "fileindex database needs an upgrade: "
		}
		h.host.SetErrorMessage(messageTitle, errors.New(problem+path))
		return
	}
	h.pendingInit = then
	h.model.FileDialog = dialog.FileDialogState{
		Open:         true,
		DialogType:   dialog.FileDialogIndexInit,
		Message:      path,
		IndexUpgrade: upgrade,
		Focus:        dlg.OK,
	}
}

// SubmitInit is the prompt's Yes: it closes the prompt and creates or upgrades the database off
// the UI thread. Other fileindex work waits on the index meanwhile.
func (h *Handler) SubmitInit() {
	then := h.pendingInit
	upgrade := h.model.FileDialog.IndexUpgrade
	h.pendingInit = nil
	h.model.FileDialog = dialog.FileDialogState{}
	if upgrade {
		h.host.SetTransientMessage("Upgrading fileindex database…", ui.MessageUrgencyInfo)
	}
	go func() {
		create := h.index.Create
		if upgrade {
			create = h.index.Upgrade
		}
		path, err := create()
		h.post(InitPayload{Path: path, Upgrade: upgrade, Err: err, then: then})
	}()
}

// HandleInit reports the created database and runs the action that asked for it.
func (h *Handler) HandleInit(p InitPayload) {
	if p.Err != nil {
		h.host.SetErrorMessage(messageTitle, p.Err)
		return
	}
	verb := "Created"
	if p.Upgrade {
		verb = "Upgraded"
	}
	h.host.SetTransientMessage(verb+" fileindex database at "+p.Path, ui.MessageUrgencySuccess)
	h.host.RefreshMetaColumns()
	if p.then != nil {
		p.then()
	}
}
