package fileindex

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/paranoidi/paras-commander/internal/config"
	fi "github.com/paranoidi/paras-commander/internal/fileindex"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

const (
	differencesTitle     = "Report differences"
	differencesTaskTitle = "fileindex differences"
)

// DifferencesPayload carries the outcome of one fileindex.report-differences task for the
// directory Root, started from PanelID at Started. Task is the job's task, which keeps the result;
// Resolved is shared with the DifferencesQueuedPayload of the same report.
type DifferencesPayload struct {
	PanelID    int
	Root       string // host path the report is for
	IndexPanel bool
	Diffs      []fi.Difference
	Err        error
	Started    time.Time
	Task       *jobs.Task
	Resolved   *bool
}

// DifferencesQueuedPayload fires config.DifferencesFastWindow after a report was requested.
type DifferencesQueuedPayload struct {
	Resolved *bool // set once the report's result has been handled
}

// ReportDifferences queues the task listing where the files under the cursor directory (else the
// panel directory) differ from the index; its result opens as a picker that jumps to a path when
// it comes soon enough, else it waits on the job (see HandleDifferences). The "queued" toast shows
// only when no result came within config.DifferencesFastWindow. The cursor entry counts when it is
// a directory other than "..", a ghost directory included.
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
	panelID, indexPanel, started := h.model.ActivePanel, p.Path.Scheme() == pathloc.SchemeFileIndex, time.Now()
	resolved := new(bool) // UI goroutine only
	var task *jobs.Task
	task = h.index.DifferencesTask(root, func(diffs []fi.Difference, err error) {
		h.post(DifferencesPayload{PanelID: panelID, Root: root, IndexPanel: indexPanel, Diffs: diffs, Err: err,
			Started: started, Task: task, Resolved: resolved})
	})
	h.addTask(task, []string{root})
	time.AfterFunc(config.DifferencesFastWindow, func() { h.post(DifferencesQueuedPayload{Resolved: resolved}) })
}

// HandleDifferencesQueued tells the user the report is still running, unless its result is in.
func (h *Handler) HandleDifferencesQueued(p DifferencesQueuedPayload) {
	if !*p.Resolved {
		h.host.SetTransientMessage(differencesTaskTitle+" queued", ui.MessageUrgencyInfo)
	}
}

// HandleDifferences shows the report's outcome. The picker opens at once when the result came
// within config.DifferencesFastWindow of the request and the user is still browsing the panel it
// was requested from; otherwise a toast points to the Jobs view. The differences stay on the job
// (Task.Results, jobs.show-results) either way. "No differences", not-indexed and error messages
// always show; a canceled report is silent.
func (h *Handler) HandleDifferences(p DifferencesPayload) {
	*p.Resolved = true
	if errors.Is(p.Err, context.Canceled) {
		return
	}
	name := filepath.Base(p.Root)
	switch {
	case errors.Is(p.Err, fi.ErrNotIndexed):
		h.host.SetTransientMessage(name+" is not in the index", ui.MessageUrgencyWarn)
	case p.Err != nil:
		h.host.SetErrorMessage(differencesTitle, p.Err)
	case len(p.Diffs) == 0:
		h.host.SetTransientMessage("No differences", ui.MessageUrgencySuccess)
	default:
		items := make([]dialog.PathPickerItem, len(p.Diffs))
		for i, d := range p.Diffs {
			items[i] = dialog.PathPickerItem{Source: d.State, Path: d.Path, PathMissing: d.State == fi.DiffMissing}
		}
		title := fmt.Sprintf("Differences under %s (%d)", name, len(items))
		show := func() { h.host.OpenFilePicker(title, items, p.IndexPanel) }
		count := fmt.Sprintf("%d %s", len(items), jobbridge.Plural(len(items), "difference", "differences"))
		p.Task.Results = &jobs.TaskResults{Label: count, Show: show}
		h.host.SyncJobsList() // the result can land after the job's terminal event
		if time.Since(p.Started) < config.DifferencesFastWindow && h.stillBrowsing(p.PanelID) {
			show()
			return
		}
		h.host.SetTransientMessage(count+" under "+name+": see Jobs view", ui.MessageUrgencySuccess)
	}
}
