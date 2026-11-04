package fileindex

import (
	"fmt"

	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

const copiesTitle = "Other copies"

// CopiesPayload carries the outcome of one fileindex.other-copies lookup for the file at Path (the
// active panel's cursor entry when it started).
type CopiesPayload struct {
	PanelID    int
	Path       string
	Name       string
	IndexPanel bool
	Files      []string
	State      fi.CopiesState
	Err        error
}

// Copies lists where else the cursor file exists according to the index and offers to jump to one.
func (h *Handler) Copies() {
	if !h.localOrIndex() {
		return
	}
	p := h.host.ActivePanel()
	entry, ok := p.CurrentEntry()
	if !ok || entry.IsDir() {
		h.host.SetTransientMessage(copiesTitle+" works on files", ui.MessageUrgencyWarn)
		return
	}
	host, err := hostPath(entry.Path)
	if err != nil {
		h.host.SetErrorMessage(copiesTitle, err)
		return
	}
	res := CopiesPayload{PanelID: h.model.ActivePanel, Path: entry.Path, Name: entry.Name, IndexPanel: p.Path.Scheme() == pathloc.SchemeFileIndex}
	h.withIndex(func() {
		go func() {
			res.Files, res.State, res.Err = h.index.OtherCopies(host)
			h.post(res)
		}()
	})
}

// HandleCopies shows the lookup unless the user has moved on since it started.
func (h *Handler) HandleCopies(p CopiesPayload) {
	if h.model.ViewMode != ui.ViewBrowser || h.model.ModalDialogOpen() || h.model.ActivePanel != p.PanelID {
		return
	}
	if cur, ok := h.host.ActivePanel().CurrentEntry(); !ok || cur.Path != p.Path {
		return
	}
	switch {
	case p.Err != nil:
		h.host.SetErrorMessage(messageTitle, p.Err)
	case p.State == fi.CopiesNotIndexed:
		h.host.SetTransientMessage("Not in the index: "+p.Name, ui.MessageUrgencyWarn)
	case p.State == fi.CopiesNotHashed:
		h.host.SetTransientMessage("Not fully hashed yet: "+p.Name, ui.MessageUrgencyWarn)
	case len(p.Files) == 0:
		h.host.SetTransientMessage("No other copies of "+p.Name, ui.MessageUrgencyInfo)
	default:
		h.host.OpenFileCopiesPicker(fmt.Sprintf("Other copies of %s (%d)", p.Name, len(p.Files)), p.Files, p.IndexPanel)
	}
}
