package app

import (
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/pathloc"
	"github.com/paranoidi/paras-commander/internal/ui"
)

func (a *App) openCopyDialog() {
	a.openTransferDialog(ui.TransferKindCopy)
}

func (a *App) openMoveDialog() {
	a.openTransferDialog(ui.TransferKindMove)
}

func absPathClean(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(abs)
}

func transferPrefilledDestination(path string) ui.FileDialogField {
	path = strings.TrimSpace(path)
	if path != "" {
		sep := string(filepath.Separator)
		if !strings.HasSuffix(path, sep) {
			path += sep
		}
	}
	rn := len([]rune(path))
	return ui.FileDialogField{
		Value:          path,
		Prefill:        path,
		Cursor:         rn,
		PrefillPending: path != "",
	}
}

func (a *App) openTransferDialog(kind ui.TransferKind) {
	passive := a.inactivePanel()
	st := ui.TransferDialogState{
		Open:         true,
		Kind:         kind,
		Destination:  transferPrefilledDestination(passive.PathString()),
		DestSubFocus: ui.TransferDestSubFocusText,
		FocusField:   0, // destination path row
	}
	if kind == ui.TransferKindCopy {
		st.PreservePermissions = a.config.Operations.PreservePermissions
		st.PreserveTimestamps = a.config.Operations.PreserveTimestamps
	}
	a.model.TransferDialog = st
	a.clearTransientMessage()
	a.armTransferDestinationValidateTimer()
}

func transferSelfCopyNewNamePrefilled(base string) ui.FileDialogField {
	rn := len([]rune(base))
	return ui.FileDialogField{
		Value:          base,
		Prefill:        base,
		Cursor:         rn,
		PrefillPending: base != "",
	}
}

// openTransferDialogSelfCopyRename opens the transfer modal directly on the "new name" step (e.g. F5/F6 onto self).
func (a *App) openTransferDialogSelfCopyRename(kind ui.TransferKind, absDestDir, sourcePath string) {
	base := filepath.Base(sourcePath)
	st := ui.TransferDialogState{
		Open:                 true,
		Kind:                 kind,
		Phase:                ui.TransferPhaseSelfCopyRename,
		Destination:          ui.FileDialogField{},
		DestSubFocus:         ui.TransferDestSubFocusText,
		SelfCopyDestDir:      absDestDir,
		SelfCopyOrigBasename: base,
		SelfCopyNewName:      transferSelfCopyNewNamePrefilled(base),
		FocusField:           0,
	}
	if kind == ui.TransferKindCopy {
		st.PreservePermissions = a.config.Operations.PreservePermissions
		st.PreserveTimestamps = a.config.Operations.PreserveTimestamps
	}
	a.model.TransferDialog = st
	a.clearTransientMessage()
}

func (a *App) closeTransferDialog() {
	a.stopTransferDestinationValidateTimer()
	a.transferDestValidateGen.Add(1)
	a.model.TransferDialog = ui.TransferDialogState{}
}

func (a *App) handleTransferDialogKey(event *tcell.EventKey) {
	d := &a.model.TransferDialog
	// Alt+O = OK, Alt+C = Cancel, Alt+P = Add paused (mnemonics; must run before field edit).
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'o', 'O':
			a.confirmTransfer()
			return
		case 'c', 'C':
			a.closeTransferDialog()
			return
		case 'p', 'P':
			a.confirmTransferPaused()
			return
		}
	}
	if event.Key() == tcell.KeyEsc {
		a.closeTransferDialog()
		return
	}
	if a.tryPathPickerHostShortcut(event) {
		return
	}
	if d.FocusField == 0 && d.Phase == ui.TransferPhaseDestination &&
		d.DestSubFocus == ui.TransferDestSubFocusText &&
		event.Key() == tcell.KeyTab && d.Destination.CompletionSuffix != "" {
		if d.Destination.AcceptCompletion() {
			a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
			a.armTransferDestinationValidateTimer()
			return
		}
		return
	}
	if d.FocusField == 0 && d.Phase == ui.TransferPhaseSelfCopyRename {
		if a.editTransferFieldKey(event, &d.SelfCopyNewName) {
			return
		}
	}
	if d.FocusField == 0 && d.Phase == ui.TransferPhaseDestination {
		if d.DestSubFocus == ui.TransferDestSubFocusPicker {
			switch event.Key() {
			case tcell.KeyLeft:
				d.DestSubFocus = ui.TransferDestSubFocusText
				runes := []rune(d.Destination.Value)
				d.Destination.Cursor = len(runes)
				return
			case tcell.KeyEnter:
				a.openPathPickerForTransfer()
				return
			case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyDown, tcell.KeyUp:
				d.DestSubFocus = ui.TransferDestSubFocusText
			default:
				return
			}
		} else {
			switch event.Key() {
			case tcell.KeyRight:
				dest := &d.Destination
				runes := []rune(dest.Value)
				c := dest.Cursor
				if c < 0 {
					c = 0
				}
				if c > len(runes) {
					c = len(runes)
				}
				// First Right on a pending placeholder commits it; second Right at EOT moves to the glyph.
				if dest.Prefill != "" && dest.PrefillPending && dest.Value == dest.Prefill && c >= len(runes) {
					dest.CommitPrefill()
					return
				}
				if c >= len(runes) {
					d.DestSubFocus = ui.TransferDestSubFocusPicker
					return
				}
				dest.MoveCursor(1)
				a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
				return
			case tcell.KeyLeft:
				d.Destination.MoveCursor(-1)
				a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
				return
			}
		}
	}
	if focus, ok := ui.TransferDialogMoveFocus(*d, d.FocusField, event.Key()); ok {
		prev := d.FocusField
		d.FocusField = focus
		if prev == 0 && focus != 0 {
			d.DestSubFocus = ui.TransferDestSubFocusText
		}
		return
	}
	if event.Key() == tcell.KeyEnter {
		tf := ui.NewTransferDialogLinearForm(ui.TransferDialogEffectiveNumContent(*d))
		if d.Phase == ui.TransferPhaseDestination && d.FocusField == 0 && d.DestSubFocus == ui.TransferDestSubFocusText {
			a.confirmTransfer()
			return
		}
		if d.Phase == ui.TransferPhaseSelfCopyRename && d.FocusField == 0 {
			a.confirmTransfer()
			return
		}
		switch d.FocusField {
		case tf.CancelIndex():
			a.closeTransferDialog()
			return
		case tf.OKIndex():
			a.confirmTransfer()
			return
		case tf.AddPausedIndex():
			a.confirmTransferPaused()
			return
		}
	}
	if d.FocusField == 0 && d.Phase != ui.TransferPhaseSelfCopyRename {
		if a.editTransferFieldKey(event, &d.Destination) {
			a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
			a.armTransferDestinationValidateTimer()
			return
		}
	}
	if d.Phase == ui.TransferPhaseDestination && d.Kind == ui.TransferKindCopy && event.Key() == tcell.KeyRune && event.Rune() == ' ' {
		switch d.FocusField {
		case 1:
			d.PreservePermissions = !d.PreservePermissions
		case 2:
			d.PreserveTimestamps = !d.PreserveTimestamps
		}
	}
}

func (a *App) editTransferFieldKey(event *tcell.EventKey, f *ui.FileDialogField) bool {
	if f == nil {
		return false
	}
	if a.tryDialogInputFieldActions(event, f) {
		return true
	}
	switch event.Key() {
	case tcell.KeyLeft:
		f.MoveCursor(-1)
		a.syncPathFieldCompletion(f, a.transferDestinationTextWidth())
		return true
	case tcell.KeyRight:
		f.MoveCursor(1)
		a.syncPathFieldCompletion(f, a.transferDestinationTextWidth())
		return true
	case tcell.KeyHome:
		f.MoveCursorStart()
		a.syncPathFieldCompletion(f, a.transferDestinationTextWidth())
		return true
	case tcell.KeyEnd:
		f.MoveCursorEnd()
		a.syncPathFieldCompletion(f, a.transferDestinationTextWidth())
		return true
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		f.Backspace()
		return true
	case tcell.KeyDelete:
		f.Delete()
		return true
	case tcell.KeyCtrlL:
		f.Clear()
		return true
	case tcell.KeyRune:
		if isPlainPrintableRune(event) {
			f.InsertRune(event.Rune())
			return true
		}
		return false
	default:
		return false
	}
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

func (a *App) confirmTransfer() {
	a.confirmTransferEnqueue(false)
}

func (a *App) confirmTransferPaused() {
	a.confirmTransferEnqueue(true)
}

func (a *App) confirmTransferEnqueue(startPaused bool) {
	d := &a.model.TransferDialog
	sources := a.activePanelSources()
	if len(sources) == 0 {
		if d.Kind == ui.TransferKindCopy {
			a.setTransientMessage("No files to copy", ui.MessageUrgencyWarn)
		} else {
			a.setTransientMessage("No files to move", ui.MessageUrgencyWarn)
		}
		return
	}

	if d.Phase == ui.TransferPhaseSelfCopyRename {
		a.confirmTransferSelfCopyRename(sources, startPaused)
		return
	}

	dest := strings.TrimSpace(d.Destination.Value)
	if dest == "" {
		a.setTransientMessage("Destination required", ui.MessageUrgencyWarn)
		return
	}
	destLoc, err := pathloc.Parse(dest)
	if err != nil {
		a.setTransientMessage("Invalid destination path", ui.MessageUrgencyWarn)
		return
	}
	absDest := destLoc.String()

	nSelf := 0
	for _, src := range sources {
		if ops.ResolvedSameAsSource(pathloc.MustParse(src), destLoc) {
			nSelf++
		}
	}
	if nSelf > 0 {
		if len(sources) > 1 {
			a.setTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		d.Phase = ui.TransferPhaseSelfCopyRename
		d.SelfCopyDestDir = absDest
		base := filepath.Base(sources[0])
		d.SelfCopyOrigBasename = base
		d.SelfCopyNewName = transferSelfCopyNewNamePrefilled(base)
		d.FocusField = 0
		a.stopTransferDestinationValidateTimer()
		a.transferDestValidateGen.Add(1)
		d.DestPathInvalid = false
		d.DestPathCheckPending = false
		return
	}

	var jobType jobs.Type
	switch d.Kind {
	case ui.TransferKindCopy:
		jobType = jobs.TypeCopy
	case ui.TransferKindMove:
		jobType = jobs.TypeMove
	default:
		a.closeTransferDialog()
		return
	}
	sourcesCopy := append([]string(nil), sources...)
	a.activePanel().ClearSelection()
	a.addTransferJob(jobType, sourcesCopy, dest, startPaused)
	a.closeTransferDialog()
	a.setTransferQueuedMessage(jobType, startPaused)
}

func (a *App) confirmTransferSelfCopyRename(sources []string, startPaused bool) {
	d := &a.model.TransferDialog
	if len(sources) != 1 {
		a.closeTransferDialog()
		return
	}
	trimmed := strings.TrimSpace(d.SelfCopyNewName.Value)
	if msg := transferBasenameIssue(trimmed); msg != "" {
		a.setTransientMessage(msg, ui.MessageUrgencyWarn)
		return
	}
	if trimmed == d.SelfCopyOrigBasename {
		a.setTransientMessage("New name must differ from the original", ui.MessageUrgencyWarn)
		return
	}

	var jobType jobs.Type
	switch d.Kind {
	case ui.TransferKindCopy:
		jobType = jobs.TypeCopy
	case ui.TransferKindMove:
		jobType = jobs.TypeMove
	default:
		a.closeTransferDialog()
		return
	}
	destDir, err := pathloc.Parse(d.SelfCopyDestDir)
	if err != nil {
		a.setTransientMessage("Invalid destination directory", ui.MessageUrgencyWarn)
		return
	}
	finalLoc, err := destDir.Join(trimmed)
	if err != nil {
		a.setTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	finalDest := finalLoc.String()
	sourcesCopy := append([]string(nil), sources...)
	a.activePanel().ClearSelection()
	a.addTransferJob(jobType, sourcesCopy, finalDest, startPaused)
	a.closeTransferDialog()
	a.setTransferQueuedMessage(jobType, startPaused)
}

func (a *App) setTransferQueuedMessage(jobType jobs.Type, paused bool) {
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
	a.setTransientMessage(msg, ui.MessageUrgencyInfo)
}

// confirmCopy confirms the unified transfer dialog when opened as copy (tests).
func (a *App) confirmCopy() {
	a.confirmTransfer()
}

func (a *App) handleConflictDialogKey(event *tcell.EventKey) {
	switch event.Key() {
	case tcell.KeyEsc:
		a.model.ConflictDialog = ui.ConflictDialogState{}
	case tcell.KeyTab:
		a.model.ConflictDialog.Focus = (a.model.ConflictDialog.Focus + 1) % 5
	case tcell.KeyBacktab:
		a.model.ConflictDialog.Focus = (a.model.ConflictDialog.Focus + 4) % 5
	case tcell.KeyLeft:
		if a.model.ConflictDialog.Focus > 0 {
			a.model.ConflictDialog.Focus--
		}
	case tcell.KeyRight:
		if a.model.ConflictDialog.Focus < 4 {
			a.model.ConflictDialog.Focus++
		}
	case tcell.KeyEnter:
		// Conflict decisions: 0=overwrite, 1=skip, 2=overwrite-all, 3=skip-all, 4=cancel
		// Our jobs engine handles these internally via the transfer func.
		// For v1, just close and re-emit via a pending decision channel (future work).
		a.model.ConflictDialog = ui.ConflictDialogState{}
	}
}

// activePanelSources returns file paths from the active panel: selected entries
// if any, otherwise the cursor entry. Returns nil if no valid sources exist.
func (a *App) activePanelSources() []string {
	source, err := ops.ResolveSource(a.activePanel())
	if err != nil {
		return nil
	}
	return ops.SourcePaths(source)
}
