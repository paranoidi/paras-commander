package dialog

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// OpenCopyDialog opens the unified transfer dialog in copy mode.
func (h *Handler) OpenCopyDialog() {
	h.openTransferDialog(dialog.TransferKindCopy)
}

// OpenMoveDialog opens the unified transfer dialog in move mode.
func (h *Handler) OpenMoveDialog() {
	h.openTransferDialog(dialog.TransferKindMove)
}

// ActivateCopyAction is the single user-facing entry point for copy (keyboard, menu, F-keys).
func (h *Handler) ActivateCopyAction() {
	h.OpenCopyDialog()
}

// ActivateMoveAction is the single user-facing entry point for move (keyboard, menu, F-keys).
// Always opens the move destination dialog, including when both panels share a path — the user
// can edit the destination; confirming the same path still routes to the self-move rename step.
// Same-directory rename is file.rename (Shift+F6), not file.move.
func (h *Handler) ActivateMoveAction() {
	h.OpenMoveDialog()
}

// TransferPrefilledDestination builds a destination FileDialogField prefilled (as a pending
// suggestion) with path, trailing-separated when non-empty. Shared by the transfer, flatten,
// and extract dialogs.
func TransferPrefilledDestination(path string) dialog.FileDialogField {
	path = strings.TrimSpace(path)
	if path != "" {
		sep := string(filepath.Separator)
		if !strings.HasSuffix(path, sep) {
			path += sep
		}
	}
	rn := len([]rune(path))
	return dialog.FileDialogField{
		Value:          path,
		Prefill:        path,
		Cursor:         rn,
		PrefillPending: path != "",
	}
}

func (h *Handler) openTransferDialog(kind dialog.TransferKind) {
	passive := h.host.InactivePanel()
	st := dialog.TransferDialogState{
		Open:        true,
		Kind:        kind,
		Destination: TransferPrefilledDestination(passive.PathString()),
		Focus:       dialog.TransferDest,
	}
	if kind == dialog.TransferKindCopy {
		h.applyCopyDefaults(&st)
	}
	// A file can't land on another file when copying a directory or several entries.
	if source, err := ops.ResolveSource(h.host.ActivePanel()); err == nil {
		st.Destination.CompletionDirsOnly = len(source.Entries) > 1 || source.Entries[0].IsDir()
	}
	if root, ok := h.multiDirSelectionCommonRoot(); ok {
		st.CommonRoot = root
		st.Entries = h.transferPreviewEntries(root)
	}
	h.model.TransferDialog = st
	h.host.ClearTransientMessage()
	h.ArmTransferDestinationValidateTimer()
}

// transferPreviewEntries builds the multi-location preview list: the active panel's
// selection resolved and labeled relative to root (not the panel's current path), so
// the preview reflects where the transfer will read from.
func (h *Handler) transferPreviewEntries(root string) []dialog.DeleteListEntry {
	source, err := ops.ResolveSource(h.host.ActivePanel())
	if err != nil {
		return nil
	}
	homeDir := h.model.UserHomeDir
	entries := make([]dialog.DeleteListEntry, len(source.Entries))
	for i, e := range source.Entries {
		entries[i] = dialog.DeleteListEntry{
			Name: dialog.DeleteListEntryName(root, homeDir, e.Path, e.Name),
			Path: e.Path,
			Type: e.Type,
		}
	}
	return entries
}

// multiDirSelectionCommonRoot returns the deepest common ancestor of the active panel's
// selected paths when they span multiple parent directories, regardless of the panel's
// current path. The transfer dialog always shows the multi-location Source/Result preview
// in that case.
func (h *Handler) multiDirSelectionCommonRoot() (string, bool) {
	root, multiDir, ok := h.host.ActivePanel().SelectedCommonRoot()
	if !ok || !multiDir {
		return "", false
	}
	return root.String(), true
}

func transferSelfCopyNewNamePrefilled(base string) dialog.FileDialogField {
	rn := len([]rune(base))
	return dialog.FileDialogField{
		Value:          base,
		Prefill:        base,
		Cursor:         rn,
		PrefillPending: base != "",
	}
}

// OpenTransferDialogSelfCopyRename opens the transfer modal directly on the "new name" step
// (e.g. F5/F6 onto self).
func (h *Handler) OpenTransferDialogSelfCopyRename(kind dialog.TransferKind, absDestDir, sourcePath string) {
	base := filepath.Base(sourcePath)
	st := dialog.TransferDialogState{
		Open:                 true,
		Kind:                 kind,
		Phase:                dialog.TransferPhaseSelfCopyRename,
		Destination:          dialog.FileDialogField{},
		SelfCopyDestDir:      absDestDir,
		SelfCopyOrigBasename: base,
		SelfCopyNewName:      transferSelfCopyNewNamePrefilled(base),
		Focus:                dialog.TransferNewName,
	}
	if kind == dialog.TransferKindCopy {
		h.applyCopyDefaults(&st)
	}
	h.model.TransferDialog = st
	h.host.ClearTransientMessage()
}

// CloseTransferDialog closes the unified transfer (copy/move) dialog and invalidates its
// debounced destination-path validation.
func (h *Handler) CloseTransferDialog() {
	h.InvalidateTransferDestValidate()
	h.invalidateRemoteFileOp()
	h.model.TransferDialog = dialog.TransferDialogState{}
	h.model.DestinationTargetPrimary = false
	h.model.DestinationTargetSecondary = false
}

// applyCopyDefaults seeds a copy dialog's checkbox state from config and the source panel.
func (h *Handler) applyCopyDefaults(st *dialog.TransferDialogState) {
	cfg := h.host.Config()
	st.PreservePermissions = cfg.Operations.PreservePermissions
	st.PreserveTimestamps = cfg.Operations.PreserveTimestamps
	st.DereferenceSymlinks = cfg.Operations.DereferenceSymlinks
	st.SourceIsRemote = h.host.ActivePanel().Path.IsRemote()
}

// transferViewContext is what dialog.TransferView needs besides the state: the screen layout, the
// home directory and the icon lead of the preview rows.
// TransferDialogSpec is the open transfer dialog's widget spec for the current screen.
func (h *Handler) TransferDialogSpec() dlg.Spec {
	return dialog.TransferView(h.model.TransferDialog, h.transferViewContext())
}

func (h *Handler) transferViewContext() dialog.TransferViewContext {
	w, ht := h.screen.Size()
	return dialog.TransferViewContext{
		Layout:      h.host.LayoutForTerminalSize(w, ht),
		UserHomeDir: h.model.UserHomeDir,
		IconLead:    ui.DialogListIconLeadingWidth(h.model.UseNerdfontIcons),
	}
}

// ApplyTransferEvent acts on a widget event (key or click): a click on an input moves its cursor,
// the checkboxes toggle, Enter in the input or OK confirms, Add paused confirms paused, Cancel
// closes.
func (h *Handler) ApplyTransferEvent(ev dlg.Event) {
	d := &h.model.TransferDialog
	if ev.Kind == dlg.EventCaret {
		switch ev.ID {
		case dialog.TransferDest:
			h.setPathFieldCursor(&d.Destination, ev.Index)
		case dialog.TransferNewName:
			d.SelfCopyNewName.SetCursor(ev.Index)
		}
		return
	}
	d.Apply(ev)
	switch ev.ID {
	case dialog.TransferDest, dialog.TransferNewName, dlg.OK:
		h.confirmTransfer()
	case dialog.TransferAddPaused:
		h.confirmTransferPaused()
	case dlg.Cancel:
		h.CloseTransferDialog()
	}
}

// HandleTransferDialogKey routes a key event for the open unified transfer (copy/move) dialog.
// Order: Alt mnemonics (options, Add paused) win over everything, also in the input
// (dlg.HandleMnemonic, which leaves focus in the input); then the
// completion dropdown, the path-picker and destination shortcuts and the
// destination's cursor keys; then the widget engine; and what it declines is text editing.
func (h *Handler) HandleTransferDialogKey(event *tcell.EventKey) {
	d := &h.model.TransferDialog
	spec := h.TransferDialogSpec()
	if ev, ok := dlg.HandleMnemonic(spec, &d.Focus, event); ok {
		h.ApplyTransferEvent(ev)
		return
	}
	if h.DestFieldTryCompletionKey(event, &d.Destination, d.FocusOnDest(), h.ArmTransferDestinationValidateTimer) {
		return
	}
	if h.TryPathPickerHostShortcut(event) {
		return
	}
	if h.TryTransferDialogDestinationShortcut(event) {
		return
	}
	if h.DestFieldNav(event, &d.Destination, d.FocusOnDest()) {
		return
	}
	if ev, ok := dlg.HandleKey(spec, &d.Focus, event); ok {
		h.ApplyTransferEvent(ev)
		return
	}
	switch {
	case d.Phase == dialog.TransferPhaseSelfCopyRename && d.FocusOnInput():
		h.editTransferFieldKey(event, &d.SelfCopyNewName)
	case d.FocusOnDest():
		if h.editTransferFieldKey(event, &d.Destination) {
			h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
			h.ArmTransferDestinationValidateTimer()
		}
	}
}

func (h *Handler) editTransferFieldKey(event *tcell.EventKey, f *dialog.FileDialogField) bool {
	return dialog.HandleFileDialogFieldKey(event, f, h.keysDialogInput, func() {
		h.SyncPathFieldCompletion(f, h.TransferDestinationTextWidth())
	})
}

func transferBasenameIssue(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "Name required"
	}
	if trimmed == "." || trimmed == ".." {
		return "Invalid name"
	}
	if filepath.Dir(trimmed) != "." {
		return "Use a single file or folder name"
	}
	return ""
}

func (h *Handler) confirmTransfer() {
	h.confirmTransferEnqueue(false)
}

func (h *Handler) confirmTransferPaused() {
	h.confirmTransferEnqueue(true)
}

func (h *Handler) confirmTransferEnqueue(startPaused bool) {
	d := &h.model.TransferDialog
	sources := h.ActivePanelSources()
	if len(sources) == 0 {
		if d.Kind == dialog.TransferKindCopy {
			h.host.SetTransientMessage("No files to copy", ui.MessageUrgencyWarn)
		} else {
			h.host.SetTransientMessage("No files to move", ui.MessageUrgencyWarn)
		}
		return
	}

	if d.Phase == dialog.TransferPhaseSelfCopyRename {
		h.confirmTransferSelfCopyRename(sources, startPaused)
		return
	}

	dest := strings.TrimSpace(d.Destination.Value)
	if dest == "" {
		h.host.SetTransientMessage("Destination required", ui.MessageUrgencyWarn)
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		h.host.SetTransientMessage("Invalid destination path", ui.MessageUrgencyWarn)
		return
	}
	var jobType jobs.Type
	switch d.Kind {
	case dialog.TransferKindCopy:
		jobType = jobs.TypeCopy
	case dialog.TransferKindMove:
		jobType = jobs.TypeMove
	default:
		h.CloseTransferDialog()
		return
	}
	flat := d.MultiLocation() && d.FlattenIntoDest
	preserve := jobs.TransferPreserve{
		PreservePermissions: d.PreservePermissions,
		PreserveTimestamps:  d.PreserveTimestamps,
		FlattenIntoDest:     flat,
		DereferenceSymlinks: d.DereferenceSymlinks,
	}
	st := transferProbeApply{
		sources:     sources,
		dest:        dest,
		destLoc:     destLoc,
		flat:        flat,
		startPaused: startPaused,
		jobType:     jobType,
		preserve:    preserve,
	}
	if h.useRemoteFileOp(dest, destLoc.String()) {
		h.startRemoteTransferProbe(st)
		return
	}
	srcLocs := make([]pathloc.Path, len(sources))
	for i, src := range sources {
		srcLocs[i] = pathloc.MustParse(src)
	}
	nSelf := ops.SelfTargetCount(srcLocs, destLoc, flat)
	h.finishTransferEnqueue(st, nSelf, false, false)
}

func (h *Handler) confirmTransferSelfCopyRename(sources []string, startPaused bool) {
	d := &h.model.TransferDialog
	if len(sources) != 1 {
		h.CloseTransferDialog()
		return
	}
	trimmed := strings.TrimSpace(d.SelfCopyNewName.Value)
	if msg := transferBasenameIssue(trimmed); msg != "" {
		h.host.SetTransientMessage(msg, ui.MessageUrgencyWarn)
		return
	}
	if trimmed == d.SelfCopyOrigBasename {
		h.host.SetTransientMessage("New name must differ from the original", ui.MessageUrgencyWarn)
		return
	}

	var jobType jobs.Type
	switch d.Kind {
	case dialog.TransferKindCopy:
		jobType = jobs.TypeCopy
	case dialog.TransferKindMove:
		jobType = jobs.TypeMove
	default:
		h.CloseTransferDialog()
		return
	}
	destDir, err := pathloc.Parse(d.SelfCopyDestDir)
	if err != nil {
		h.host.SetTransientMessage("Invalid destination directory", ui.MessageUrgencyWarn)
		return
	}
	finalLoc, err := destDir.Join(trimmed)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	finalDest := finalLoc.String()
	sourcesCopy := append([]string(nil), sources...)
	preserve := jobs.TransferPreserve{
		PreservePermissions: d.PreservePermissions,
		PreserveTimestamps:  d.PreserveTimestamps,
		DereferenceSymlinks: d.DereferenceSymlinks,
	}
	h.AddTransferJob(jobType, sourcesCopy, finalDest, startPaused, preserve)
	h.host.ActivePanel().ClearSelection()
	h.CloseTransferDialog()
	h.setTransferQueuedMessage(jobType, startPaused)
}

func (h *Handler) setTransferQueuedMessage(jobType jobs.Type, paused bool) {
	var msg string
	if jobType == jobs.TypeCopy {
		if paused {
			msg = "Copy queued (paused)"
		} else {
			msg = "Copy queued"
		}
	} else if paused {
		msg = "Move queued (paused)"
	} else {
		msg = "Move queued"
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencyInfo)
}

// ConfirmCopy confirms the unified transfer dialog when opened as copy (tests).
func (h *Handler) ConfirmCopy() {
	h.confirmTransfer()
}

// ActivePanelSources returns file paths from the active panel: selected entries
// if any, otherwise the cursor entry. Returns nil if no valid sources exist.
func (h *Handler) ActivePanelSources() []string {
	source, err := ops.ResolveSource(h.host.ActivePanel())
	if err != nil {
		return nil
	}
	return ops.SourcePaths(source)
}
