package commands

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OpenRunForEachDialog opens the Run-for-each command dialog for the active panel's selection
// (or the whole listing when nothing is selected).
func (h *Handler) OpenRunForEachDialog() {
	if h.model.ViewMode != ui.ViewBrowser {
		return
	}
	active := h.host.ActivePanel()
	src, err := ops.ResolveSource(active)
	if err != nil {
		h.host.SetErrorMessage("Run for each", err)
		return
	}
	entries := append([]localfs.Entry(nil), src.Entries...)
	dir := active.PathString()
	msg := "Runs once per selected item. Command must include %f (iterated item path).\n" +
		"Other macros: %d active dir, %F/%D other panel, %t/%T tagged paths.\n" +
		"Do not wrap % macros in quotes.\n" +
		">> | && etc. run via sh -c; otherwise argv is parsed without a shell.\n" +
		"Check \"Run in each selected directory\" (Alt+R) to cd into each selected directory " +
		"instead (directories only; %f becomes optional)."
	fields := []dialog.FileDialogField{{Label: "Command", Value: "", Cursor: 0}}
	h.model.FileDialog = dialog.FileDialogState{
		Open:              true,
		DialogType:        dialog.FileDialogRunForEach,
		Fields:            fields,
		FocusedField:      0,
		Message:           msg,
		RunForEachEntries: entries,
		RunForEachDir:     dir,
		RunForEachPools:   h.workPools.Names(),
		RunForEachPool:    "",
	}
	h.RecomputeRunForEachValidation()
	h.host.ClearTransientMessage()
}

// ExecuteRunForEach validates and starts the run-for-each batch from the open dialog.
func (h *Handler) ExecuteRunForEach() {
	fd := &h.model.FileDialog
	field := h.host.FocusedFileDialogField()
	if field == nil {
		h.host.CloseFileDialog()
		return
	}
	cmdLine := strings.TrimSpace(field.Value)
	entries := append([]localfs.Entry(nil), fd.RunForEachEntries...)
	workDir := fd.RunForEachDir
	poolName := strings.TrimSpace(fd.RunForEachPool)
	inDirs := fd.RunForEachInDirs
	usePTY := fd.RunForEachPTY
	active := h.host.ActivePanel()
	other := h.host.InactivePanel()
	h.RecomputeRunForEachValidation()
	if strings.TrimSpace(fd.RunForEachCommandError) != "" {
		return
	}
	h.host.CloseFileDialog()
	h.StartRunForEachBatch(RunForEachBatchSpec{
		Kind:            ui.CommandRunKindRunForEach,
		Entries:         entries,
		AllowDirs:       true,
		AllowFiles:      !inDirs,
		WorkDir:         workDir,
		PerEntryWorkDir: inDirs,
		PoolName:        poolName,
		Background:      false,
		PTY:             usePTY,
		NotifyLabel:     "Run for each",
		BuildItem: func(ent localfs.Entry) (RunForEachBuiltItem, error) {
			return BuildRunForEachItem(cmdLine, ent, active, other, false, !inDirs)
		},
	})
}

// RecomputeRunForEachValidation refreshes the run-for-each dialog's inline command validation
// error, if the dialog is open.
func (h *Handler) RecomputeRunForEachValidation() {
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogRunForEach || len(d.Fields) == 0 {
		return
	}
	preview, msg := validateRunForEachCommand(
		strings.TrimSpace(d.Fields[0].Value),
		d.RunForEachEntries,
		h.host.ActivePanel(),
		h.host.InactivePanel(),
		d.RunForEachInDirs,
	)
	d.RunForEachPreview = preview
	d.RunForEachCommandError = msg
	d.Fields[0].InputInvalid = msg != ""
}

func validateRunForEachCommand(cmdLine string, entries []localfs.Entry, active, other *panel.State, inDirs bool) (preview, errMsg string) {
	if cmdLine == "" {
		return "", "Command is empty"
	}
	if inDirs {
		for _, e := range entries {
			if e.Type != localfs.EntryDirectory {
				return "", "Selection must contain only directories"
			}
		}
	}
	ent := localfs.Entry{}
	if len(entries) > 0 {
		ent = entries[0]
	} else if active != nil {
		if e, ok := active.CurrentEntry(); ok {
			ent = e
		}
	}
	if ent.Path == "" && active != nil {
		ent.Path = active.PathString()
	}
	built, err := BuildRunForEachItem(cmdLine, ent, active, other, false, !inDirs)
	if err != nil {
		return "", err.Error()
	}
	return built.UserLine, ""
}
