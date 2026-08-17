package dialog

import (
	"fmt"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/filenameenc"
	"github.com/paranoidi/paras-commander/internal/jobbridge"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// CloseFileDialog closes the file dialog and clears its disk-usage scan reconcile cache.
func (h *Handler) CloseFileDialog() {
	h.ClearDeleteDialogReconcileCache()
	h.model.FileDialog = dialog.FileDialogState{}
}

// TryDispatchFileOps handles file-operation dialog openers and copy/move/duplicate actions.
func (h *Handler) TryDispatchFileOps(actionID string) bool {
	activePanel := h.host.ActivePanel()
	switch actionID {
	case keymap.ActionFileRename:
		h.OpenRenameDialog(activePanel)
	case keymap.ActionFileDelete:
		h.OpenDeleteDialog(activePanel)
	case keymap.ActionFileMkdir:
		h.OpenMkdirDialog(false)
	case keymap.ActionFileMkdirOpenInOther:
		h.OpenMkdirDialog(true)
	case keymap.ActionFileChmod:
		h.openChmodDialog(activePanel)
	case keymap.ActionFileChown:
		h.openChownDialog(activePanel)
	case keymap.ActionFileSymlink:
		h.openSymlinkDialog(activePanel)
	case keymap.ActionFileHardlink:
		h.openHardlinkDialog(activePanel)
	case keymap.ActionFileExtract:
		h.OpenExtractDialog(activePanel)
	case keymap.ActionFileFlatten:
		h.OpenFlattenDialog()
	case keymap.ActionCopy:
		h.ActivateCopyAction()
	case keymap.ActionFileDuplicate:
		h.activateDuplicateAction()
	case keymap.ActionMove:
		h.ActivateMoveAction()
	case keymap.ActionFileRunForEach:
		if h.model.ViewMode == ui.ViewBrowser {
			h.commands.OpenRunForEachDialog()
		}
	default:
		return false
	}
	return true
}

// RefreshBothPanels re-lists both panels, walking up when their directory vanished (e.g. a
// completed job removed the active panel's cwd), then applies any pending post-duplicate focus
// and refreshes the quick-view preview.
func (h *Handler) RefreshBothPanels() {
	viewportRows := h.host.ActiveViewportRows()
	_ = h.model.Primary.RefreshOrNavigateToExistingAncestor(viewportRows)
	_ = h.model.Secondary.RefreshOrNavigateToExistingAncestor(viewportRows)
	// Immediate attempt for the (still common) case where nothing is actually async — e.g. no
	// scheduler wired. When the reload above is async, this attempt is a harmless no-op (entries
	// are still stale) and ReconcilePendingPanelFocus retries it once the reload lands.
	h.applyPendingPanelFocus()
	h.preview.ApplyQuickViewPreviewImmediately()
}

// ReconcilePendingPanelFocus retries a select-and-center scheduled by rename/mkdir/duplicate
// whose triggering directory reload hadn't landed yet. Called from App.reconcileAfterEvent after
// every event (including the reload's own async completion), matching that chokepoint's
// idempotent, retry-until-it-holds design.
func (h *Handler) ReconcilePendingPanelFocus() {
	h.applyPendingPanelFocus()
}

// OpenRenameDialog opens the mass-rename dialog when p has a selection, otherwise the
// single-entry rename dialog.
func (h *Handler) OpenRenameDialog(p *panel.State) {
	if len(p.SelectedPaths) > 0 {
		h.OpenMassRenameDialog(p)
		return
	}
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		h.host.SetErrorMessage("Rename", err)
		return
	}
	name := entry.Name
	nameRunes := len([]rune(name))
	encCands := filenameenc.DetectCandidates(name)
	renameEnc := make([]dialog.RenameEncodingCandidate, 0, len(encCands))
	for _, c := range encCands {
		renameEnc = append(renameEnc, dialog.RenameEncodingCandidate{Label: c.Label, UTF8: c.UTF8})
	}
	fields := []dialog.FileDialogField{
		{Label: "Name", Value: name, Prefill: name, Cursor: nameRunes, PrefillPending: true},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:                     true,
		DialogType:               dialog.FileDialogRename,
		Fields:                   fields,
		RenamePhase:              dialog.RenamePhaseMain,
		RenameSlugifySep:         dialog.RenameSlugifyDot,
		RenameFocusAfter:         h.host.Config().Operations.RenameFocusAfter,
		RenameEncodingCandidates: renameEnc,
		RenameEncodingSelected:   0,
	}
}

// OpenMkdirDialog opens the mkdir dialog. openInInactive requests navigating the inactive
// panel to the newly created directory on success.
func (h *Handler) OpenMkdirDialog(openInInactive bool) {
	p := h.host.ActivePanel()
	name := ""
	if entry, ok := p.CurrentEntry(); ok {
		name = entry.Name
	}
	cursor := 0
	if name != "" {
		cursor = len([]rune(name))
	}
	pending := name != ""
	fields := []dialog.FileDialogField{
		{Label: "Directory name", Value: name, Prefill: name, Cursor: cursor, PrefillPending: pending},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:                true,
		DialogType:          dialog.FileDialogMkdir,
		Fields:              fields,
		MkdirShowActions:    len(p.SelectedPaths) > 0,
		MkdirAction:         dialog.MkdirActionCreate,
		MkdirOpenInInactive: openInInactive,
	}
}

// OpenDeleteDialog opens the delete confirmation for p's resolved source (selection or cursor entry).
func (h *Handler) OpenDeleteDialog(p *panel.State) {
	source, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Delete", err)
		return
	}
	panelPath := p.PathString()
	homeDir := h.model.UserHomeDir
	entries := make([]dialog.DeleteListEntry, len(source.Entries))
	for i, e := range source.Entries {
		entries[i] = dialog.DeleteListEntry{
			Name: dialog.DeleteListEntryName(panelPath, homeDir, e.Path, e.Name),
			Path: e.Path,
			Type: e.Type,
		}
	}
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	h.ClearDeleteDialogReconcileCache()
	h.invalidateDeleteDialogDiskCache(p, source)
	h.deleteDialogSelGen = p.SelectionDerivedGen()
	h.deleteDialogPanelPath = panelPath
	h.deleteDialogPrunedPaths = pruned
	fd := dialog.FileDialogState{
		Open:          true,
		DialogType:    dialog.FileDialogDelete,
		DeleteSummary: h.DeleteDialogSummary(p, source),
		DeleteEntries: entries,
		FocusedField:  1, // No (safe default); Yes stays index 0.
	}
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(h.model.ShowFileIcons))
	h.model.FileDialog = fd
	h.ReconcileDeleteDialogScans()
}

// OpenDeleteDialogForPreviewedFile shows the delete confirm dialog for the file currently open
// in the fullscreen viewer (F8), never the active panel's selection.
func (h *Handler) OpenDeleteDialogForPreviewedFile() {
	path := h.model.FullscreenFilePreview.Path
	if path == "" {
		return
	}
	entry, err := localfs.EntryFromPath(path)
	if err != nil {
		h.host.SetErrorMessage("Delete", err)
		return
	}
	entries := []dialog.DeleteListEntry{{
		Name: h.model.FullscreenFilePreview.TitleBase,
		Path: entry.Path,
		Type: entry.Type,
	}}
	fd := dialog.FileDialogState{
		Open:          true,
		DialogType:    dialog.FileDialogDelete,
		DeleteSummary: ui.FormatDeleteImpactSummary(1, entry.Size, false, ""),
		DeleteEntries: entries,
		FocusedField:  1, // No (safe default); Yes stays index 0.
	}
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(h.model.ShowFileIcons))
	h.model.FileDialog = fd
}

// PromptDanglingDirDelete is the jobsctrl.Host.PromptDanglingDirDelete implementation: it opens
// the delete confirmation for directories a completed move/delete job left empty, unless
// another modal already owns the screen — in that case the dirs simply stay visible in the
// panel and a transient message is surfaced instead of clobbering whatever the user has open.
func (h *Handler) PromptDanglingDirDelete(dirs []string) {
	if len(dirs) == 0 {
		return
	}
	if h.model.AnyModalOpen() {
		h.host.SetTransientMessage(fmt.Sprintf("%d empty %s left behind", len(dirs), jobbridge.Plural(len(dirs), "directory", "directories")), ui.MessageUrgencyInfo)
		return
	}
	h.openDanglingDirsDeleteDialog(dirs)
}

// openDanglingDirsDeleteDialog opens the standard delete confirmation for directories left
// empty by a completed move/delete job, reusing the browser dialog (list + summary + Yes/No).
// Modeled on the dedup empty-dirs confirm; defaults to Yes (index 0) — removing already-empty
// directories is low-risk, same rationale as dedup.
func (h *Handler) openDanglingDirsDeleteDialog(dirs []string) {
	entries := make([]dialog.DeleteListEntry, len(dirs))
	for i, d := range dirs {
		entries[i] = dialog.DeleteListEntry{Name: d, Path: d, Type: localfs.EntryDirectory}
	}
	fd := dialog.FileDialogState{
		Open:               true,
		DialogType:         dialog.FileDialogDelete,
		DeleteSummary:      fmt.Sprintf("%d %s left empty", len(dirs), jobbridge.Plural(len(dirs), "directory", "directories")),
		DeleteEntries:      entries,
		FocusedField:       0, // Yes default
		DeleteDanglingDirs: true,
	}
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(h.model.ShowFileIcons))
	h.model.FileDialog = fd
}

func (h *Handler) openChmodDialog(p *panel.State) {
	if p.Path.IsRemote() {
		h.host.SetTransientMessage("Chmod is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	_, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Chmod", err)
		return
	}
	fields := []dialog.FileDialogField{
		{Label: "Mode", Value: ""},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogChmod,
		Fields:     fields,
	}
	h.model.FileDialog.Fields[0].Cursor = 0
}

func (h *Handler) openChownDialog(p *panel.State) {
	if p.Path.IsRemote() {
		h.host.SetTransientMessage("Chown is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	_, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Chown", err)
		return
	}
	fields := []dialog.FileDialogField{
		{Label: "User", Value: ""},
		{Label: "Group", Value: ""},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogChown,
		Fields:     fields,
	}
}

func (h *Handler) openSymlinkDialog(p *panel.State) {
	if p.Path.IsRemote() {
		h.host.SetTransientMessage("Symlink is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		h.host.SetErrorMessage("Symlink", err)
		return
	}
	targetPath := entry.Path
	defaultLink := filepath.Join(h.host.InactivePanel().PathString(), entry.Name)
	fields := []dialog.FileDialogField{
		{Label: "Target", Value: targetPath, Cursor: len([]rune(targetPath)), PathPicker: true},
		{Label: "Link path", Value: defaultLink, Cursor: len([]rune(defaultLink)), PathPicker: true},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogSymlink,
		Fields:     fields,
	}
	h.SyncFocusedFileDialogPathFieldCompletion()
}

func (h *Handler) openHardlinkDialog(p *panel.State) {
	if p.Path.IsRemote() {
		h.host.SetTransientMessage("Hardlink is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		h.host.SetErrorMessage("Hardlink", err)
		return
	}
	sourcePath := entry.Path
	defaultDest := filepath.Join(h.host.InactivePanel().PathString(), entry.Name)
	fields := []dialog.FileDialogField{
		{Label: "Source", Value: sourcePath, Cursor: len([]rune(sourcePath)), PathPicker: true},
		{Label: "New path", Value: defaultDest, Cursor: len([]rune(defaultDest)), PathPicker: true},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogHardlink,
		Fields:     fields,
	}
	h.SyncFocusedFileDialogPathFieldCompletion()
}

func (h *Handler) ExecuteFileDialog() {
	switch h.model.FileDialog.DialogType {
	case dialog.FileDialogRunForEach:
		h.commands.ExecuteRunForEach()
	case dialog.FileDialogMassRename:
		h.ExecuteMassRename()
	case dialog.FileDialogRename:
		h.executeRename()
	case dialog.FileDialogDuplicate:
		h.executeDuplicate()
	case dialog.FileDialogMkdir:
		h.executeMkdir()
	case dialog.FileDialogDelete:
		h.ExecuteDelete()
	case dialog.FileDialogChmod:
		h.executeChmod()
	case dialog.FileDialogChown:
		h.executeChown()
	case dialog.FileDialogSymlink:
		h.executeSymlink()
	case dialog.FileDialogHardlink:
		h.executeHardlink()
	case dialog.FileDialogExtract:
		h.ExecuteExtract()
	case dialog.FileDialogAddBookmark:
		h.ExecuteAddBookmark()
	case dialog.FileDialogSFTPPassword:
		h.host.ExecuteSFTPPassword()
	default:
		h.CloseFileDialog()
	}
}

func (h *Handler) executeRename() {
	p := h.host.ActivePanel()
	d := &h.model.FileDialog
	if len(d.Fields) == 0 {
		h.CloseFileDialog()
		return
	}
	newName := d.Fields[0].Value
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		h.host.SetErrorMessage("Rename source", err)
		h.CloseFileDialog()
		return
	}
	plan, err := ops.PlanRename(entry, newName, p.PathString())
	if err != nil {
		h.host.SetErrorMessage("Rename", err)
		h.CloseFileDialog()
		return
	}
	if err := ops.ExecuteRename(plan); err != nil {
		h.host.SetErrorMessage("Rename failed", err)
		h.CloseFileDialog()
		return
	}
	focusAfter := h.model.FileDialog.RenameFocusAfter
	panelDir := p.Path
	panelID := h.model.ActivePanel
	listDir := p.PathString()
	h.CloseFileDialog()
	if focusAfter {
		h.schedulePanelFocus(panelID, listDir, plan.NewName)
	}
	h.RefreshBothPanels()
	h.host.ActivePanel().AddRenameMarks(panelDir, []string{plan.NewName})
	h.host.SetTransientMessage(fmt.Sprintf("Renamed to %s", plan.NewName), ui.MessageUrgencyInfo)
}

func (h *Handler) executeMkdir() {
	p := h.host.ActivePanel()
	d := &h.model.FileDialog
	if len(d.Fields) == 0 {
		h.CloseFileDialog()
		return
	}
	input := d.Fields[0].Value
	action := dialog.MkdirActionCreate
	if d.MkdirShowActions {
		action = d.MkdirAction
	}
	openInInactive := d.MkdirOpenInInactive
	priorEntryName := ""
	if entry, ok := p.CurrentEntry(); ok {
		priorEntryName = entry.Name
	}

	plan, err := ops.PlanMkdir(input, p.PathString())
	if err != nil {
		h.host.SetErrorMessage("Mkdir", err)
		h.CloseFileDialog()
		return
	}

	// For copy/move post-actions, resolve sources up-front so a missing/empty
	// selection fails fast without leaving an empty directory behind.
	var sources []string
	if action == dialog.MkdirActionCreateCopySelect || action == dialog.MkdirActionCreateMoveSelect {
		src, srcErr := ops.ResolveSource(p)
		if srcErr != nil {
			h.host.SetErrorMessage("Mkdir source", srcErr)
			h.CloseFileDialog()
			return
		}
		if src.Kind != ops.SourceSelected {
			h.host.SetErrorMessage("Mkdir", &ops.Error{Op: "mkdir", Text: "no files selected for transfer"})
			h.CloseFileDialog()
			return
		}
		sources = ops.SourcePaths(src)
	}

	if err := ops.ExecuteMkdir(plan); err != nil {
		h.host.SetErrorMessage("Mkdir failed", err)
		h.CloseFileDialog()
		return
	}

	createdName := plan.Name
	if loc, err := pathloc.Parse(plan.Path); err == nil {
		createdName = loc.Base()
	}
	panelID := h.model.ActivePanel
	listDir := p.PathString()
	h.CloseFileDialog()
	active := h.host.ActivePanel()
	viewportRows := h.host.ActiveViewportRows()
	if openInInactive {
		if priorEntryName != "" && priorEntryName != createdName {
			h.schedulePanelFocusScroll(panelID, listDir, priorEntryName, false)
		}
		// The createdName-under-cursor fallback (step off the just-created directory before
		// reload) still runs synchronously below on the panel's pre-reload cursor state — no
		// specific target name to defer, so there is nothing to schedule here.
	} else {
		h.schedulePanelFocusScroll(panelID, listDir, createdName, false)
	}
	h.RefreshBothPanels()
	if openInInactive {
		if entry, ok := active.CurrentEntry(); ok && entry.Name == createdName {
			if !active.SelectVisibleEntry("..") {
				for i := 0; i < active.VisibleEntryCount(); i++ {
					e, _, ok := active.VisibleEntry(i)
					if ok && e.Name != createdName {
						active.Cursor = i
						break
					}
				}
			}
		}
		active.EnsureCursorInViewport(viewportRows)
		if err := h.host.NavigatePanelToPath(h.host.InactivePanelID(), plan.Path, ""); err != nil {
			h.host.SetErrorMessage("Mkdir", err)
			return
		}
	}

	switch action {
	case dialog.MkdirActionCreate:
		h.host.SetTransientMessage(fmt.Sprintf("Created directory %s", plan.Name), ui.MessageUrgencyInfo)
	case dialog.MkdirActionCreateCopySelect:
		h.host.ActivePanel().ClearSelection()
		h.AddTransferJob(jobs.TypeCopy, sources, plan.Path, false, h.TransferPreserveFromConfig())
		h.host.SetTransientMessage(fmt.Sprintf("Created %s; copy queued (%d %s)", plan.Name, len(sources), jobbridge.Plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	case dialog.MkdirActionCreateMoveSelect:
		h.host.ActivePanel().ClearSelection()
		h.AddTransferJob(jobs.TypeMove, sources, plan.Path, false, h.TransferPreserveFromConfig())
		h.host.SetTransientMessage(fmt.Sprintf("Created %s; move queued (%d %s)", plan.Name, len(sources), jobbridge.Plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	}
}

// ExecuteDelete runs the delete confirmation dialog's Yes action: dangling-dirs cleanup,
// dedup-view delete, fullscreen-preview delete, or the standard panel-selection delete,
// depending on which context the dialog was opened from.
func (h *Handler) ExecuteDelete() {
	// Dangling-dirs cleanup: the dialog lists directories a completed move/delete
	// job left empty, not a fresh panel/preview selection. Must run before the
	// ViewMode branches below, which re-resolve sources from panel state.
	if h.model.FileDialog.DeleteDanglingDirs {
		paths := make([]string, len(h.model.FileDialog.DeleteEntries))
		for i, e := range h.model.FileDialog.DeleteEntries {
			paths[i] = e.Path
		}
		h.CloseFileDialog()
		h.jobs.EnqueueDeleteJob(paths, false, false)
		h.host.SetTransientMessage(fmt.Sprintf("Delete queued (%d %s)", len(paths), jobbridge.Plural(len(paths), "directory", "directories")), ui.MessageUrgencyInfo)
		return
	}
	// Dedup delete: the dialog overlays the dedup view; route to the handler
	// which enqueues the job and prunes the marked rows.
	if h.model.ViewMode == ui.ViewDedup {
		h.CloseFileDialog()
		h.openDedupEmptyDirsConfirm()
		return
	}
	if h.model.ViewMode == ui.ViewFilePreview {
		path := h.model.FullscreenFilePreview.Path
		h.CloseFileDialog()
		h.jobs.EnqueueDeleteJob([]string{path}, false, true)
		h.preview.CloseFilePreviewFullscreen()
		h.host.SetTransientMessage("Delete queued (1 item)", ui.MessageUrgencyInfo)
		return
	}
	p := h.host.ActivePanel()
	source, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Delete source", err)
		h.CloseFileDialog()
		return
	}
	cfg := h.host.Config()
	_, err = ops.PlanDelete(source, cfg.Operations.ConfirmDelete, cfg.Operations.DeleteMode)
	if err != nil {
		h.host.SetErrorMessage("Delete", err)
		h.CloseFileDialog()
		return
	}
	p.ClearSelection()
	h.CloseFileDialog()
	sources := make([]string, len(source.Entries))
	for i, e := range source.Entries {
		sources[i] = e.Path
	}
	h.jobs.EnqueueDeleteJob(sources, false, true)
	n := len(sources)
	delNoun := "items"
	if n == 1 {
		delNoun = "item"
	}
	h.host.SetTransientMessage(fmt.Sprintf("Delete queued (%d %s)", n, delNoun), ui.MessageUrgencyInfo)
}

func (h *Handler) executeChmod() {
	p := h.host.ActivePanel()
	field := h.FocusedField()
	if field == nil {
		h.CloseFileDialog()
		return
	}
	source, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Chmod source", err)
		h.CloseFileDialog()
		return
	}
	plan, err := ops.PlanChmod(source, field.Value)
	if err != nil {
		h.host.SetErrorMessage("Chmod", err)
		h.CloseFileDialog()
		return
	}
	if err := ops.ExecuteChmod(plan); err != nil {
		h.host.SetErrorMessage("Chmod failed", err)
		h.CloseFileDialog()
		return
	}
	h.CloseFileDialog()
	h.RefreshBothPanels()
	h.host.SetTransientMessage(fmt.Sprintf("Changed mode to %s on %d item(s)", plan.ModeStr, len(plan.Entries)), ui.MessageUrgencyInfo)
}

func (h *Handler) executeChown() {
	p := h.host.ActivePanel()
	if len(h.model.FileDialog.Fields) < 2 {
		h.CloseFileDialog()
		return
	}
	user := h.model.FileDialog.Fields[0].Value
	group := h.model.FileDialog.Fields[1].Value
	source, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Chown source", err)
		h.CloseFileDialog()
		return
	}
	plan, err := ops.PlanChown(source, user, group)
	if err != nil {
		h.host.SetErrorMessage("Chown", err)
		h.CloseFileDialog()
		return
	}
	if err := ops.ExecuteChown(plan); err != nil {
		h.host.SetErrorMessage("Chown failed", err)
		h.CloseFileDialog()
		return
	}
	h.CloseFileDialog()
	h.RefreshBothPanels()
	h.host.SetTransientMessage(fmt.Sprintf("Changed owner on %d item(s)", len(plan.Entries)), ui.MessageUrgencyInfo)
}

func (h *Handler) executeSymlink() {
	p := h.host.ActivePanel()
	if len(h.model.FileDialog.Fields) < 2 {
		h.CloseFileDialog()
		return
	}
	target := h.model.FileDialog.Fields[0].Value
	linkPath := h.model.FileDialog.Fields[1].Value
	plan, err := ops.PlanSymlink(target, linkPath, p.PathString(), h.host.InactivePanel().PathString())
	if err != nil {
		h.host.SetErrorMessage("Symlink", err)
		h.CloseFileDialog()
		return
	}
	if err := ops.ExecuteSymlink(plan); err != nil {
		h.host.SetErrorMessage("Symlink failed", err)
		h.CloseFileDialog()
		return
	}
	h.CloseFileDialog()
	h.RefreshBothPanels()
	h.host.SetTransientMessage(fmt.Sprintf("Created symlink: %s -> %s", filepath.Base(plan.LinkPath), plan.TargetSrc), ui.MessageUrgencyInfo)
}

func (h *Handler) executeHardlink() {
	p := h.host.ActivePanel()
	if len(h.model.FileDialog.Fields) < 2 {
		h.CloseFileDialog()
		return
	}
	source := h.model.FileDialog.Fields[0].Value
	newPath := h.model.FileDialog.Fields[1].Value
	plan, err := ops.PlanHardlink(source, newPath, p.PathString(), h.host.InactivePanel().PathString())
	if err != nil {
		h.host.SetErrorMessage("Hardlink", err)
		h.CloseFileDialog()
		return
	}
	if err := ops.ExecuteHardlink(plan); err != nil {
		h.host.SetErrorMessage("Hardlink failed", err)
		h.CloseFileDialog()
		return
	}
	h.CloseFileDialog()
	h.RefreshBothPanels()
	h.host.SetTransientMessage(fmt.Sprintf("Created hardlink: %s", filepath.Base(plan.NewPath)), ui.MessageUrgencyInfo)
}
