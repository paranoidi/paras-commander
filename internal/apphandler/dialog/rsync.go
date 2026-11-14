package dialog

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/cmdrun"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/rsync"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// rsyncError is an option problem as shown to the user (a message, so it starts with a capital).
type rsyncError string

func (e rsyncError) Error() string { return string(e) }

// rsyncChangesDialogWidth and rsyncChangesDialogHeight size the command-output dialog that shows
// a dry run's itemized changes.
const (
	rsyncChangesDialogWidth  = "80%"
	rsyncChangesDialogHeight = "70%"
)

// RsyncDryRunPayload carries the itemized changes of a finished dry-run rsync job back to the
// main goroutine; Task is the job's task, which keeps them as its results.
type RsyncDryRunPayload struct {
	Task    *jobs.Task
	Changes []string
}

// rsyncMemory is the Rsync dialog's last-used options, kept for the session (never persisted) and
// offered again the next time the dialog opens. The destination is not part of it: that follows
// the inactive panel.
type rsyncMemory struct {
	options                   rsync.Options
	exclude, bandwidth, extra string
}

// newRsyncMemory is what the first Rsync dialog of a session offers: archive mode.
func newRsyncMemory() rsyncMemory { return rsyncMemory{options: rsync.Options{Archive: true}} }

// OpenRsyncDialog opens the Rsync dialog for p's selection (or cursor entry), prefilling the
// destination with the inactive panel's path and the options with the last used ones. Without
// rsync on PATH it shows an error instead.
func (h *Handler) OpenRsyncDialog(p *panel.State) {
	lookPath := h.lookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	if _, err := lookPath("rsync"); err != nil {
		h.host.SetErrorMessage("Rsync", rsync.ErrNotInstalled)
		return
	}
	source, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Rsync", err)
		return
	}
	var paths []string
	allDirs := true
	for _, e := range source.Entries {
		if e.Name == ".." {
			continue
		}
		paths = append(paths, e.Path)
		allDirs = allDirs && e.IsDir()
	}
	if len(paths) == 0 {
		h.host.SetTransientMessage("Nothing to sync", ui.MessageUrgencyWarn)
		return
	}
	if msg := panel.GhostRefusal(p.GhostSources(paths...)); msg != "" {
		h.host.SetTransientMessage(msg, ui.MessageUrgencyWarn)
		return
	}
	locs, err := pathloc.ParseAll(paths)
	if err != nil {
		h.host.SetErrorMessage("Rsync", err)
		return
	}

	mem := h.rsyncLast
	opts := mem.options
	opts.ContentsOnly = opts.ContentsOnly && allDirs
	text := func(label, value string) dialog.FileDialogField {
		return dialog.FileDialogField{Label: label, Value: value, Cursor: len([]rune(value))}
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogRsync,
		Focus:      dialog.FileFieldID(dialog.RsyncFieldDest), // the Mode radios above it are not where typing starts
		Fields: []dialog.FileDialogField{
			DialogDestinationField(h.host.InactivePanel().PathString(), true),
			text("Exclude", mem.exclude),
			text("Bandwidth limit (KiB/s)", mem.bandwidth),
			text("Extra arguments", mem.extra),
		},
		RsyncSources: pathloc.Locals(locs),
		RsyncAllDirs: allDirs,
		RsyncOptions: opts,
	}
	h.updateRsyncPreview()
	h.SyncFocusedFileDialogPathFieldCompletion()
	h.host.ClearTransientMessage()
}

// rsyncRun is the Rsync dialog's options built into what a job needs.
type rsyncRun struct {
	options rsync.Options
	sources []pathloc.Path
	dest    pathloc.Path
}

// buildRsyncRun builds the run from the dialog's state, or says why it cannot: this is the one
// place that turns the dialog into rsync.Options, for the command preview and for the job alike.
func (h *Handler) buildRsyncRun() (rsyncRun, error) {
	d := h.model.FileDialog
	field := func(i int) string { return strings.TrimSpace(d.Fields[i].Value) }
	if field(dialog.RsyncFieldDest) == "" {
		return rsyncRun{}, rsyncError("Destination required")
	}
	dest, err := h.resolveDestination(field(dialog.RsyncFieldDest))
	if err != nil {
		return rsyncRun{}, rsyncError("Invalid destination path")
	}
	if dest.Scheme() == pathloc.SchemeFileIndex {
		return rsyncRun{}, rsyncError(ops.IndexReadOnlyRefusal)
	}
	sources := d.RsyncSources
	targets, err := rsync.Resolve(sources, dest)
	if err != nil {
		return rsyncRun{}, err
	}
	o := d.RsyncOptions
	o.Sources, o.Dest, o.SSH = targets.Sources, targets.Dest, targets.SSH
	o.Exclude = strings.Fields(field(dialog.RsyncFieldExclude))
	if bw := field(dialog.RsyncFieldBandwidth); bw != "" {
		n, err := strconv.Atoi(bw)
		if err != nil || n <= 0 {
			return rsyncRun{}, rsyncError("Bandwidth limit must be a whole number of KiB/s")
		}
		o.BwLimit = n
	}
	if extra := field(dialog.RsyncFieldExtra); extra != "" {
		args, err := cmdrun.ParseCommandArgv(extra)
		if err != nil {
			return rsyncRun{}, rsyncError("Extra arguments: " + strings.TrimPrefix(err.Error(), "parse command: "))
		}
		o.Extra = args
	}
	return rsyncRun{options: o, sources: sources, dest: dest}, nil
}

// updateRsyncPreview recomputes the open dialog's command preview row (or the error that stands
// in its place) and marks the panel the destination points at, with the warning styles for a
// mirror, which deletes there. Called after every change to the dialog.
func (h *Handler) updateRsyncPreview() {
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogRsync {
		return
	}
	run, err := h.buildRsyncRun()
	d.RsyncPreview, d.RsyncError = nil, ""
	if err != nil {
		d.RsyncError = err.Error()
	} else {
		d.RsyncPreview = rsync.DisplayArgs(rsync.BuildArgv(run.options))
	}
	h.updateDestinationTargetPanels(h.host.ActivePanel().PathString(), d.Fields[dialog.RsyncFieldDest].Value, d.RsyncOptions.Mode == rsync.ModeMirror)
}

// ExecuteRsync runs the Rsync dialog's OK action: queues an rsync job for the sources gathered when
// the dialog opened. The dialog stays open, with a message, when the options do not build.
func (h *Handler) ExecuteRsync() {
	run, err := h.buildRsyncRun()
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	d := h.model.FileDialog
	h.rsyncLast = rsyncMemory{
		options:   d.RsyncOptions,
		exclude:   d.Fields[dialog.RsyncFieldExclude].Value,
		bandwidth: d.Fields[dialog.RsyncFieldBandwidth].Value,
		extra:     d.Fields[dialog.RsyncFieldExtra].Value,
	}
	h.CloseFileDialog()

	var task *jobs.Task
	var onDryRun func([]string)
	if run.options.DryRun {
		onDryRun = func(changes []string) { h.postRsyncDryRun(RsyncDryRunPayload{Task: task, Changes: changes}) }
	}
	task = rsync.NewTask(run.options, onDryRun)
	h.host.ActivePanel().ClearSelection()
	h.jobs.AddRsyncJob(task, run.sources, run.dest)
	msg := "Rsync queued"
	if run.options.DryRun {
		msg = "Rsync dry run queued"
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
}

func (h *Handler) postRsyncDryRun(p RsyncDryRunPayload) {
	if h.screen != nil {
		_ = h.screen.PostEvent(tcell.NewEventInterrupt(p))
	}
}

// ApplyRsyncDryRun keeps a dry run's itemized changes as its job's results (the jobs view's Show
// results opens them in the command output dialog) and tells the user. Main goroutine only.
func (h *Handler) ApplyRsyncDryRun(p RsyncDryRunPayload) {
	n := len(p.Changes)
	if n == 0 {
		p.Task.Results = &jobs.TaskResults{Label: "No changes"}
		h.jobs.SyncJobsList()
		h.host.SetTransientMessage("Rsync dry run: nothing would change", ui.MessageUrgencySuccess)
		return
	}
	label := fmt.Sprintf("%d %s", n, jobbridge.Plural(n, "change", "changes"))
	changes := p.Changes
	p.Task.Results = &jobs.TaskResults{Label: label, Show: func() {
		h.model.CommandOutputDialog = dialog.CommandOutputDialogState{
			Open:       true,
			Title:      "Rsync dry run: " + label,
			Lines:      changes,
			PrefWidth:  rsyncChangesDialogWidth,
			PrefHeight: rsyncChangesDialogHeight,
		}
	}}
	h.jobs.SyncJobsList()
	h.host.SetTransientMessage("Rsync dry run: "+label+" (see Jobs view)", ui.MessageUrgencySuccess)
}

// SetLookPathForTest replaces the lookup of the rsync binary (tests only).
func (h *Handler) SetLookPathForTest(fn func(string) (string, error)) { h.lookPath = fn }
