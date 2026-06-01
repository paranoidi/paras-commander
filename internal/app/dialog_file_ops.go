package app

import (
	"fmt"
	"path/filepath"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) closeFileDialog() {
	a.model.FileDialog = ui.FileDialogState{}
}

func (a *App) refreshBothPanels() {
	viewportRows := a.activeViewportRows()
	_ = a.model.Left.Refresh(viewportRows)
	_ = a.model.Right.Refresh(viewportRows)
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
	fields := []ui.FileDialogField{
		{Label: "Name", Value: name, Prefill: name, Cursor: nameRunes, PrefillPending: true},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:             true,
		DialogType:       ui.FileDialogRename,
		Fields:           fields,
		RenamePhase:      ui.RenamePhaseMain,
		RenameSlugifySep: ui.RenameSlugifyDot,
		RenameFocusAfter: a.config.Operations.RenameFocusAfter,
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
	fields := []ui.FileDialogField{
		{Label: "Directory name", Value: name, Prefill: name, Cursor: cursor, PrefillPending: pending},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:                true,
		DialogType:          ui.FileDialogMkdir,
		Fields:              fields,
		MkdirShowActions:    len(p.SelectedPaths) > 0,
		MkdirAction:         ui.MkdirActionCreate,
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
	entries := make([]ui.DeleteListEntry, len(source.Entries))
	for i, e := range source.Entries {
		entries[i] = ui.DeleteListEntry{
			Name: ui.DeleteListEntryName(panelPath, homeDir, e.Path, e.Name),
			Path: e.Path,
			Type: e.Type,
		}
	}
	a.deleteDialogScanFP = ""
	a.model.FileDialog = ui.FileDialogState{
		Open:          true,
		DialogType:    ui.FileDialogDelete,
		DeleteSummary: a.deleteDialogSummary(p, source),
		DeleteEntries: entries,
		FocusedField:  1, // No (safe default); Yes stays index 0.
	}
	a.reconcileDeleteDialogScans()
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
	fields := []ui.FileDialogField{
		{Label: "Mode", Value: ""},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogChmod,
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
	fields := []ui.FileDialogField{
		{Label: "User", Value: ""},
		{Label: "Group", Value: ""},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogChown,
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
	fields := []ui.FileDialogField{
		{Label: "Target", Value: targetPath, Cursor: len([]rune(targetPath)), PathPicker: true},
		{Label: "Link path", Value: defaultLink, Cursor: len([]rune(defaultLink)), PathPicker: true},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogSymlink,
		Fields:     fields,
	}
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
	fields := []ui.FileDialogField{
		{Label: "Source", Value: sourcePath, Cursor: len([]rune(sourcePath)), PathPicker: true},
		{Label: "New path", Value: defaultDest, Cursor: len([]rune(defaultDest)), PathPicker: true},
	}
	a.model.FileDialog = ui.FileDialogState{
		Open:       true,
		DialogType: ui.FileDialogHardlink,
		Fields:     fields,
	}
}

func (a *App) executeFileDialog() {
	switch a.model.FileDialog.DialogType {
	case ui.FileDialogRunForEach:
		a.executeRunForEach()
	case ui.FileDialogMassRename:
		a.executeMassRename()
	case ui.FileDialogRename:
		a.executeRename()
	case ui.FileDialogMkdir:
		a.executeMkdir()
	case ui.FileDialogDelete:
		a.executeDelete()
	case ui.FileDialogChmod:
		a.executeChmod()
	case ui.FileDialogChown:
		a.executeChown()
	case ui.FileDialogSymlink:
		a.executeSymlink()
	case ui.FileDialogHardlink:
		a.executeHardlink()
	case ui.FileDialogExtract:
		a.executeExtract()
	case ui.FileDialogAddBookmark:
		a.executeAddBookmark()
	case ui.FileDialogSFTPPassword:
		a.executeSFTPPassword()
	default:
		a.closeFileDialog()
	}
}

func (a *App) executeRename() {
	p := a.activePanel()
	field := a.focusedField()
	if field == nil {
		a.closeFileDialog()
		return
	}
	newName := field.Value
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
	a.closeFileDialog()
	a.refreshBothPanels()
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
	action := ui.MkdirActionCreate
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
	if action == ui.MkdirActionCreateCopySelect || action == ui.MkdirActionCreateMoveSelect {
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
	if openInInactive {
		active := a.activePanel()
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
		a.activePanel().SelectVisibleEntry(createdName)
	}
	if openInInactive {
		if err := a.navigatePanelToDirectory(a.inactivePanelID(), plan.Path, ""); err != nil {
			a.setErrorMessage("Mkdir", err)
			return
		}
	}

	switch action {
	case ui.MkdirActionCreate:
		a.setTransientMessage(fmt.Sprintf("Created directory %s", plan.Name), ui.MessageUrgencyInfo)
	case ui.MkdirActionCreateCopySelect:
		a.activePanel().ClearSelection()
		a.addTransferJob(jobs.TypeCopy, sources, plan.Path, false)
		a.setTransientMessage(fmt.Sprintf("Created %s; copy queued (%d %s)", plan.Name, len(sources), plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	case ui.MkdirActionCreateMoveSelect:
		a.activePanel().ClearSelection()
		a.addTransferJob(jobs.TypeMove, sources, plan.Path, false)
		a.setTransientMessage(fmt.Sprintf("Created %s; move queued (%d %s)", plan.Name, len(sources), plural(len(sources), "file", "files")), ui.MessageUrgencyInfo)
	}
}

func (a *App) executeDelete() {
	p := a.activePanel()
	source, err := ops.ResolveSource(p)
	if err != nil {
		a.setErrorMessage("Delete source", err)
		a.closeFileDialog()
		return
	}
	_, err = ops.PlanDelete(source, a.config.ConfirmDelete, a.config.DeleteMode)
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
	a.enqueueDeleteJob(sources)
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
