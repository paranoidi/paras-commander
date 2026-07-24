package app

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

func (a *App) closeFileDialog() {
	a.clearDeleteDialogReconcileCache()
	a.model.FileDialog = dialog.FileDialogState{}
}

// tryDispatchFileOps handles file-operation dialog openers and copy/move/duplicate actions.
func (a *App) tryDispatchFileOps(actionID string) bool {
	activePanel := a.activePanel()
	switch actionID {
	case keymap.ActionFileRename:
		a.openRenameDialog(activePanel)
	case keymap.ActionFileDelete:
		a.openDeleteDialog(activePanel)
	case keymap.ActionFileMkdir:
		a.openMkdirDialog(false)
	case keymap.ActionFileMkdirOpenInOther:
		a.openMkdirDialog(true)
	case keymap.ActionFileChmod:
		a.openChmodDialog(activePanel)
	case keymap.ActionFileChown:
		a.openChownDialog(activePanel)
	case keymap.ActionFileSymlink:
		a.openSymlinkDialog(activePanel)
	case keymap.ActionFileHardlink:
		a.openHardlinkDialog(activePanel)
	case keymap.ActionFileExtract:
		a.openExtractDialog(activePanel)
	case keymap.ActionFileFlatten:
		a.openFlattenDialog()
	case keymap.ActionCopy:
		a.activateCopyAction()
	case keymap.ActionFileDuplicate:
		a.activateDuplicateAction()
	case keymap.ActionMove:
		a.activateMoveAction()
	case keymap.ActionFileRunForEach:
		if a.model.ViewMode == ui.ViewBrowser {
			a.commandsCtrl.OpenRunForEachDialog()
		}
	default:
		return false
	}
	return true
}

func (a *App) refreshBothPanels() {
	// Both panels walk up when their directory vanished (identical to Refresh
	// otherwise): jobs can remove the active panel's cwd too, e.g. the
	// dangling-dirs cleanup deleting the directory the user is standing in.
	viewportRows := a.activeViewportRows()
	_ = a.model.Primary.RefreshOrNavigateToExistingAncestor(viewportRows)
	_ = a.model.Secondary.RefreshOrNavigateToExistingAncestor(viewportRows)
	a.applyDuplicateFocusPending()
	a.applyQuickViewPreviewImmediately()
}

func (a *App) openRenameDialog(p *panel.State) {
	if len(p.SelectedPaths) > 0 {
		a.openMassRenameDialog(p)
		return
	}
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Rename", err)
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
	a.model.FileDialog = dialog.FileDialogState{
		Open:                     true,
		DialogType:               dialog.FileDialogRename,
		Fields:                   fields,
		RenamePhase:              dialog.RenamePhaseMain,
		RenameSlugifySep:         dialog.RenameSlugifyDot,
		RenameFocusAfter:         a.config.Operations.RenameFocusAfter,
		RenameEncodingCandidates: renameEnc,
		RenameEncodingSelected:   0,
	}
}

func (a *App) openMkdirDialog(openInInactive bool) {
	p := a.activePanel()
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
	a.model.FileDialog = dialog.FileDialogState{
		Open:                true,
		DialogType:          dialog.FileDialogMkdir,
		Fields:              fields,
		MkdirShowActions:    len(p.SelectedPaths) > 0,
		MkdirAction:         dialog.MkdirActionCreate,
		MkdirOpenInInactive: openInInactive,
	}
}

func (a *App) openDeleteDialog(p *panel.State) {
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Delete", err)
		return
	}
	panelPath := p.PathString()
	homeDir := a.model.UserHomeDir
	entries := make([]dialog.DeleteListEntry, len(source.Entries))
	for i, e := range source.Entries {
		entries[i] = dialog.DeleteListEntry{
			Name: dialog.DeleteListEntryName(panelPath, homeDir, e.Path, e.Name),
			Path: e.Path,
			Type: e.Type,
		}
	}
	pruned := panel.PruneNestedPaths(ops.SourcePaths(source))
	a.clearDeleteDialogReconcileCache()
	a.invalidateDeleteDialogDiskCache(p, source)
	a.deleteDialogSelGen = p.SelectionDerivedGen()
	a.deleteDialogPanelPath = panelPath
	a.deleteDialogPrunedPaths = pruned
	fd := dialog.FileDialogState{
		Open:          true,
		DialogType:    dialog.FileDialogDelete,
		DeleteSummary: a.deleteDialogSummary(p, source),
		DeleteEntries: entries,
		FocusedField:  1, // No (safe default); Yes stays index 0.
	}
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(a.model.ShowFileIcons))
	a.model.FileDialog = fd
	a.reconcileDeleteDialogScans()
}

// openDeleteDialogForPreviewedFile shows the delete confirm dialog for the file currently open
// in the fullscreen viewer (F8), never the active panel's selection.
func (a *App) openDeleteDialogForPreviewedFile() {
	path := a.model.FullscreenFilePreview.Path
	if path == "" {
		return
	}
	entry, err := localfs.EntryFromPath(path)
	if err != nil {
		a.setErrorMessage("Delete", err)
		return
	}
	entries := []dialog.DeleteListEntry{{
		Name: a.model.FullscreenFilePreview.TitleBase,
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
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(a.model.ShowFileIcons))
	a.model.FileDialog = fd
}

// promptDanglingDirDelete is the jobsHost.PromptDanglingDirDelete implementation: it
// opens the delete confirmation for directories a completed move/delete job left
// empty, unless another modal already owns the screen — in that case the dirs simply
// stay visible in the panel and we surface a transient message instead of clobbering
// whatever the user has open.
func (a *App) promptDanglingDirDelete(dirs []string) {
	if len(dirs) == 0 {
		return
	}
	if a.model.AnyModalOpen() {
		a.setTransientMessage(fmt.Sprintf("%d empty %s left behind", len(dirs), jobbridge.Plural(len(dirs), "directory", "directories")), ui.MessageUrgencyInfo)
		return
	}
	a.openDanglingDirsDeleteDialog(dirs)
}

// openDanglingDirsDeleteDialog opens the standard delete confirmation for directories
// left empty by a completed move/delete job, reusing the browser dialog (list +
// summary + Yes/No). Modeled on openDedupDeleteDialog; defaults to Yes (index 0) —
// removing already-empty directories is low-risk, same rationale as dedup.
func (a *App) openDanglingDirsDeleteDialog(dirs []string) {
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
	fd.DeleteLayoutMinWidth = dialog.ComputeDeleteDialogLayoutMinWidth(fd, ui.DialogListIconLeadingWidth(a.model.ShowFileIcons))
	a.model.FileDialog = fd
}

func (a *App) openChmodDialog(p *panel.State) {
	if p.Path.IsRemote() {
		a.setTransientMessage("Chmod is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	_, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chmod", err)
		return
	}
	fields := []dialog.FileDialogField{
		{Label: "Mode", Value: ""},
	}
	a.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogChmod,
		Fields:     fields,
	}
	a.model.FileDialog.Fields[0].Cursor = 0
}

func (a *App) openChownDialog(p *panel.State) {
	if p.Path.IsRemote() {
		a.setTransientMessage("Chown is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	_, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chown", err)
		return
	}
	fields := []dialog.FileDialogField{
		{Label: "User", Value: ""},
		{Label: "Group", Value: ""},
	}
	a.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogChown,
		Fields:     fields,
	}
}

func (a *App) openSymlinkDialog(p *panel.State) {
	if p.Path.IsRemote() {
		a.setTransientMessage("Symlink is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Symlink", err)
		return
	}
	targetPath := entry.Path
	defaultLink := filepath.Join(a.inactivePanel().PathString(), entry.Name)
	fields := []dialog.FileDialogField{
		{Label: "Target", Value: targetPath, Cursor: len([]rune(targetPath)), PathPicker: true},
		{Label: "Link path", Value: defaultLink, Cursor: len([]rune(defaultLink)), PathPicker: true},
	}
	a.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogSymlink,
		Fields:     fields,
	}
	a.syncFocusedFileDialogPathFieldCompletion()
}

func (a *App) openHardlinkDialog(p *panel.State) {
	if p.Path.IsRemote() {
		a.setTransientMessage("Hardlink is not available on remote panels", ui.MessageUrgencyWarn)
		return
	}
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Hardlink", err)
		return
	}
	sourcePath := entry.Path
	defaultDest := filepath.Join(a.inactivePanel().PathString(), entry.Name)
	fields := []dialog.FileDialogField{
		{Label: "Source", Value: sourcePath, Cursor: len([]rune(sourcePath)), PathPicker: true},
		{Label: "New path", Value: defaultDest, Cursor: len([]rune(defaultDest)), PathPicker: true},
	}
	a.model.FileDialog = dialog.FileDialogState{
		Open:       true,
		DialogType: dialog.FileDialogHardlink,
		Fields:     fields,
	}
	a.syncFocusedFileDialogPathFieldCompletion()
}

func (a *App) executeFileDialog() {
	switch a.model.FileDialog.DialogType {
	case dialog.FileDialogRunForEach:
		a.commandsCtrl.ExecuteRunForEach()
	case dialog.FileDialogMassRename:
		a.executeMassRename()
	case dialog.FileDialogRename:
		a.executeRename()
	case dialog.FileDialogDuplicate:
		a.executeDuplicate()
	case dialog.FileDialogMkdir:
		a.executeMkdir()
	case dialog.FileDialogDelete:
		a.executeDelete()
	case dialog.FileDialogChmod:
		a.executeChmod()
	case dialog.FileDialogChown:
		a.executeChown()
	case dialog.FileDialogSymlink:
		a.executeSymlink()
	case dialog.FileDialogHardlink:
		a.executeHardlink()
	case dialog.FileDialogExtract:
		a.executeExtract()
	case dialog.FileDialogAddBookmark:
		a.executeAddBookmark()
	case dialog.FileDialogSFTPPassword:
		a.executeSFTPPassword()
	default:
		a.closeFileDialog()
	}
}

func (a *App) executeRename() {
	p := a.activePanel()
	d := &a.model.FileDialog
	if len(d.Fields) == 0 {
		a.closeFileDialog()
		return
	}
	newName := d.Fields[0].Value
	entry, err := ops.ResolveSourceSingle(p)
	if err != nil {
		a.setErrorMessage("Rename source", err)
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanRename(entry, newName, p.PathString())
	if err != nil {
		a.setErrorMessage("Rename", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteRename(plan); err != nil {
		a.setErrorMessage("Rename failed", err)
		a.closeFileDialog()
		return
	}
	focusAfter := a.model.FileDialog.RenameFocusAfter
	panelDir := p.Path
	a.closeFileDialog()
	a.refreshBothPanels()
	a.activePanel().AddRenameMarks(panelDir, []string{plan.NewName})
	if focusAfter {
		a.activePanel().SelectVisibleEntryCentered(plan.NewName, a.activeViewportRows())
	}
	a.setTransientMessage(fmt.Sprintf("Renamed to %s", plan.NewName), ui.MessageUrgencyInfo)
}

func (a *App) executeMkdir() {
	p := a.activePanel()
	d := &a.model.FileDialog
	if len(d.Fields) == 0 {
		a.closeFileDialog()
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
		a.setErrorMessage("Mkdir", err)
		a.closeFileDialog()
		return
	}

	// For copy/move post-actions, resolve sources up-front so a missing/empty
	// selection fails fast without leaving an empty directory behind.
	var sources []string
	if action == dialog.MkdirActionCreateCopySelect || action == dialog.MkdirActionCreateMoveSelect {
		src, srcErr := ops.ResolveSource(p)
		if srcErr != nil {
			a.setErrorMessage("Mkdir source", srcErr)
			a.closeFileDialog()
			return
		}
		if src.Kind != ops.SourceSelected {
			a.setErrorMessage("Mkdir", &ops.Error{Op: "mkdir", Text: "no files selected for transfer"})
			a.closeFileDialog()
			return
		}
		sources = ops.SourcePaths(src)
	}

	if err := ops.ExecuteMkdir(plan); err != nil {
		a.setErrorMessage("Mkdir failed", err)
		a.closeFileDialog()
		return
	}

	a.closeFileDialog()
	a.refreshBothPanels()
	createdName := plan.Name
	if loc, err := pathloc.Parse(plan.Path); err == nil {
		createdName = loc.Base()
	}
	active := a.activePanel()
	viewportRows := a.activeViewportRows()
	if openInInactive {
		if priorEntryName != "" && priorEntryName != createdName {
			active.SelectVisibleEntry(priorEntryName)
		} else if entry, ok := active.CurrentEntry(); ok && entry.Name == createdName {
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
	} else {
		active.SelectVisibleEntryInViewport(createdName, viewportRows)
	}
	if openInInactive {
		active.EnsureCursorInViewport(viewportRows)
	}
	if openInInactive {
		if err := a.navigatePanelToDirectory(a.inactivePanelID(), plan.Path, ""); err != nil {
			a.setErrorMessage("Mkdir", err)
			return
		}
	}

	switch action {
	case dialog.MkdirActionCreate:
		a.setTransientMessage(fmt.Sprintf("Created directory %s", plan.Name), ui.MessageUrgencyInfo)
	case dialog.MkdirActionCreateCopySelect:
		a.activePanel().ClearSelection()
		a.addTransferJob(jobs.TypeCopy, sources, plan.Path, false, a.transferPreserveFromConfig())
		a.setTransientMessage(fmt.Sprintf("Created %s; copy queued (%d %s)", plan.Name, len(sources), jobbridge.Plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	case dialog.MkdirActionCreateMoveSelect:
		a.activePanel().ClearSelection()
		a.addTransferJob(jobs.TypeMove, sources, plan.Path, false, a.transferPreserveFromConfig())
		a.setTransientMessage(fmt.Sprintf("Created %s; move queued (%d %s)", plan.Name, len(sources), jobbridge.Plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	}
}

func (a *App) executeDelete() {
	// Dangling-dirs cleanup: the dialog lists directories a completed move/delete
	// job left empty, not a fresh panel/preview selection. Must run before the
	// ViewMode branches below, which re-resolve sources from panel state.
	if a.model.FileDialog.DeleteDanglingDirs {
		paths := make([]string, len(a.model.FileDialog.DeleteEntries))
		for i, e := range a.model.FileDialog.DeleteEntries {
			paths[i] = e.Path
		}
		a.closeFileDialog()
		a.jobsCtrl.EnqueueDeleteJob(paths, false, false)
		a.setTransientMessage(fmt.Sprintf("Delete queued (%d %s)", len(paths), jobbridge.Plural(len(paths), "directory", "directories")), ui.MessageUrgencyInfo)
		return
	}
	// Dedup delete: the dialog overlays the dedup view; route to the handler
	// which enqueues the job and prunes the marked rows.
	if a.model.ViewMode == ui.ViewDedup {
		a.closeFileDialog()
		a.openDedupEmptyDirsConfirm()
		return
	}
	if a.model.ViewMode == ui.ViewFilePreview {
		path := a.model.FullscreenFilePreview.Path
		a.closeFileDialog()
		a.jobsCtrl.EnqueueDeleteJob([]string{path}, false, true)
		a.closeFilePreviewFullscreen()
		a.setTransientMessage("Delete queued (1 item)", ui.MessageUrgencyInfo)
		return
	}
	p := a.activePanel()
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Delete source", err)
		a.closeFileDialog()
		return
	}
	_, err = ops.PlanDelete(source, a.config.Operations.ConfirmDelete, a.config.Operations.DeleteMode)
	if err != nil {
		a.setErrorMessage("Delete", err)
		a.closeFileDialog()
		return
	}
	p.ClearSelection()
	a.closeFileDialog()
	sources := make([]string, len(source.Entries))
	for i, e := range source.Entries {
		sources[i] = e.Path
	}
	a.jobsCtrl.EnqueueDeleteJob(sources, false, true)
	n := len(sources)
	delNoun := "items"
	if n == 1 {
		delNoun = "item"
	}
	a.setTransientMessage(fmt.Sprintf("Delete queued (%d %s)", n, delNoun), ui.MessageUrgencyInfo)
}

func (a *App) executeChmod() {
	p := a.activePanel()
	field := a.focusedField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chmod source", err)
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanChmod(source, field.Value)
	if err != nil {
		a.setErrorMessage("Chmod", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteChmod(plan); err != nil {
		a.setErrorMessage("Chmod failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Changed mode to %s on %d item(s)", plan.ModeStr, len(plan.Entries)), ui.MessageUrgencyInfo)
}

func (a *App) executeChown() {
	p := a.activePanel()
	if len(a.model.FileDialog.Fields) < 2 {
		a.closeFileDialog()
		return
	}
	user := a.model.FileDialog.Fields[0].Value
	group := a.model.FileDialog.Fields[1].Value
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Chown source", err)
		a.closeFileDialog()
		return
	}
	plan, err := ops.PlanChown(source, user, group)
	if err != nil {
		a.setErrorMessage("Chown", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteChown(plan); err != nil {
		a.setErrorMessage("Chown failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Changed owner on %d item(s)", len(plan.Entries)), ui.MessageUrgencyInfo)
}

func (a *App) executeSymlink() {
	p := a.activePanel()
	if len(a.model.FileDialog.Fields) < 2 {
		a.closeFileDialog()
		return
	}
	target := a.model.FileDialog.Fields[0].Value
	linkPath := a.model.FileDialog.Fields[1].Value
	plan, err := ops.PlanSymlink(target, linkPath, p.PathString(), a.inactivePanel().PathString())
	if err != nil {
		a.setErrorMessage("Symlink", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteSymlink(plan); err != nil {
		a.setErrorMessage("Symlink failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Created symlink: %s -> %s", filepath.Base(plan.LinkPath), plan.TargetSrc), ui.MessageUrgencyInfo)
}

func (a *App) executeHardlink() {
	p := a.activePanel()
	if len(a.model.FileDialog.Fields) < 2 {
		a.closeFileDialog()
		return
	}
	source := a.model.FileDialog.Fields[0].Value
	newPath := a.model.FileDialog.Fields[1].Value
	plan, err := ops.PlanHardlink(source, newPath, p.PathString(), a.inactivePanel().PathString())
	if err != nil {
		a.setErrorMessage("Hardlink", err)
		a.closeFileDialog()
		return
	}
	if err := ops.ExecuteHardlink(plan); err != nil {
		a.setErrorMessage("Hardlink failed", err)
		a.closeFileDialog()
		return
	}
	a.closeFileDialog()
	a.refreshBothPanels()
	a.setTransientMessage(fmt.Sprintf("Created hardlink: %s", filepath.Base(plan.NewPath)), ui.MessageUrgencyInfo)
}
