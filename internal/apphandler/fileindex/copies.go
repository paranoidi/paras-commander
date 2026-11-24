package fileindex

import (
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

const (
	copiesTitle         = "Other copies"
	copiesOutsideTitle  = "Copies outside directory"
	findDuplicatesTitle = "Find duplicates"
)

// CopiesPayload carries the outcome of one fileindex.other-copies lookup for the file at Path (the
// active panel's cursor entry when it started).
type CopiesPayload struct {
	PanelID    int
	Path       string
	Name       string
	IndexPanel bool
	Files      []fi.CopyFile
	State      fi.CopiesState
	Err        error
}

// Copies lists where else the cursor file exists according to the index, in the Other copies view.
func (h *Handler) Copies() {
	if !h.localOrIndex() {
		return
	}
	p := h.host.FocusedPanel()
	entry, ok := p.CurrentEntry()
	if !ok || entry.IsDir() {
		h.host.SetTransientMessage(copiesTitle+" works on files", ui.MessageUrgencyWarn)
		return
	}
	host, ok := pathloc.HostOf(entry.Path)
	if !ok {
		h.host.SetErrorMessage(copiesTitle, errNoHostPath)
		return
	}
	res := CopiesPayload{PanelID: h.model.FocusedPanelID(), Path: entry.Path, Name: entry.Name, IndexPanel: p.Path.Scheme() == pathloc.SchemeFileIndex}
	h.WithIndex(func() {
		go func() {
			res.Files, res.State, res.Err = h.index.OtherCopies(host)
			h.post(res)
		}()
	})
}

// CopiesOutside opens the duplicates view over the files in the active panel's directory that
// also exist elsewhere in the index.
func (h *Handler) CopiesOutside() {
	if !h.localOrIndex() || h.refuseTagList() {
		return
	}
	dir, ok := pathloc.HostOf(h.host.FocusedPanel().PathString())
	if !ok {
		h.host.SetErrorMessage(copiesOutsideTitle, errNoHostPath)
		return
	}
	if dir == "/" { // the index root: everything is inside it
		h.host.SetTransientMessage("The index root has nothing outside it", ui.MessageUrgencyWarn)
		return
	}
	h.host.OpenDedupOutside(dir)
}

// FindDuplicates opens the duplicates view over the indexed files in the active panel's
// directory, also from a local panel (never reading the disk).
func (h *Handler) FindDuplicates() {
	if !h.localOrIndex() || h.refuseTagList() {
		return
	}
	dir, ok := pathloc.HostOf(h.host.FocusedPanel().PathString())
	if !ok {
		h.host.SetErrorMessage(findDuplicatesTitle, errNoHostPath)
		return
	}
	h.host.OpenDedupIndexed(dir)
}

// stillBrowsing reports whether a result started from panelID can still be shown: a file list has
// focus, no dialog is open and panelID is still the focused panel.
func (h *Handler) stillBrowsing(panelID int) bool {
	return h.model.FileListFocused() && !h.model.ModalDialogOpen() && h.model.FocusedPanelID() == panelID
}

// HandleCopies shows the lookup unless the user has moved on since it started.
func (h *Handler) HandleCopies(p CopiesPayload) {
	if !h.stillBrowsing(p.PanelID) {
		return
	}
	if cur, ok := h.host.FocusedPanel().CurrentEntry(); !ok || cur.Path != p.Path {
		return
	}
	switch {
	case p.Err != nil:
		h.host.SetErrorMessage(messageTitle, p.Err)
	case p.State == fi.CopiesNotIndexed:
		h.host.SetTransientMessage("Not in the index: "+p.Name, ui.MessageUrgencyWarn)
	case p.State == fi.CopiesNotHashed:
		h.host.SetTransientMessage("Not fully hashed yet: "+p.Name, ui.MessageUrgencyWarn)
	case p.State == fi.CopiesEmpty:
		h.host.SetTransientMessage("Empty file, copies not tracked: "+p.Name, ui.MessageUrgencyInfo)
	case len(p.Files) == 0:
		h.host.SetTransientMessage("No other copies of "+p.Name, ui.MessageUrgencyInfo)
	default:
		h.host.OpenCopiesView(p.Name, p.IndexPanel, p.Files)
	}
}
