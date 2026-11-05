package fileindex

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const differencesTitle = "Report differences"

// DifferencesPayload carries the outcome of one fileindex.report-differences task for the
// directory Root, started from PanelID.
type DifferencesPayload struct {
	PanelID    int
	Root       string // host path the report is for
	IndexPanel bool
	Diffs      []fi.Difference
	Err        error
}

// ReportDifferences queues the task listing where the files under the cursor directory (else the
// panel directory) differ from the index; its result opens as a picker that jumps to a path. The
// cursor entry counts when it is a directory other than "..", a ghost directory included.
func (h *Handler) ReportDifferences() {
	if !h.localOrIndex() {
		return
	}
	p := h.host.ActivePanel()
	target := p.PathString()
	if e, ok := p.CurrentEntry(); ok && e.IsDir() && e.Name != ".." {
		target = e.Path
	}
	root, err := hostPath(target)
	if err != nil {
		h.host.SetErrorMessage(differencesTitle, err)
		return
	}
	panelID, indexPanel := h.model.ActivePanel, p.Path.Scheme() == pathloc.SchemeFileIndex
	task := h.index.DifferencesTask(root, func(diffs []fi.Difference, err error) {
		h.post(DifferencesPayload{PanelID: panelID, Root: root, IndexPanel: indexPanel, Diffs: diffs, Err: err})
	})
	h.queueTask("fileindex differences", task, []string{root})
}

// HandleDifferences shows the report unless the user has moved on since the task started: it is
// dropped when the job was canceled, the browser is no longer shown, a dialog is open or the
// active panel changed.
func (h *Handler) HandleDifferences(p DifferencesPayload) {
	if errors.Is(p.Err, context.Canceled) {
		return
	}
	if !h.stillBrowsing(p.PanelID) {
		return
	}
	name := filepath.Base(p.Root)
	switch {
	case errors.Is(p.Err, fi.ErrNotIndexed):
		h.host.SetTransientMessage(name+" is not in the index", ui.MessageUrgencyWarn)
	case p.Err != nil:
		h.host.SetErrorMessage(differencesTitle, p.Err)
	case len(p.Diffs) == 0:
		h.host.SetTransientMessage("No differences under "+name, ui.MessageUrgencyInfo)
	default:
		items := make([]dialog.PathPickerItem, len(p.Diffs))
		for i, d := range p.Diffs {
			items[i] = dialog.PathPickerItem{Source: d.State, Path: d.Path, PathMissing: d.State == fi.DiffMissing}
		}
		h.host.OpenFilePicker(fmt.Sprintf("Differences under %s (%d)", name, len(items)), items, p.IndexPanel)
	}
}
