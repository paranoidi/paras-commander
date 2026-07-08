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
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

func (a *App) openCopyDialog() {
	a.openTransferDialog(dialog.TransferKindCopy)
}

func (a *App) openMoveDialog() {
	a.openTransferDialog(dialog.TransferKindMove)
}

// activateCopyAction is the single user-facing entry point for copy (keyboard, menu, F-keys).
func (a *App) activateCopyAction() {
	a.openCopyDialog()
}

// activateMoveAction is the single user-facing entry point for move (keyboard, menu, F-keys).
// With no selection, a single cursor item already shown in the passive panel opens Rename instead.
func (a *App) activateMoveAction() {
	p := a.activePanel()
	if len(p.SelectedPaths) == 0 {
		if entry, ok := p.CurrentEntry(); ok {
			dest := a.inactivePanel().PathString()
			if entry.Path == filepath.Join(dest, entry.Name) {
				a.openRenameDialog(p)
				return
			}
		}
	}
	a.openMoveDialog()
}

func absPathClean(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return filepath.Clean(abs)
}

func transferPrefilledDestination(path string) dialog.FileDialogField {
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

func (a *App) openTransferDialog(kind dialog.TransferKind) {
	if root, ok := a.multiDirSelectionCommonRoot(); ok {
		a.model.AmbiguousTransfer = dialog.AmbiguousTransferState{Open: true, CommonRoot: root}
		a.clearTransientMessage()
		return
	}
	passive := a.inactivePanel()
	st := dialog.TransferDialogState{
		Open:         true,
		Kind:         kind,
		Destination:  transferPrefilledDestination(passive.PathString()),
		DestSubFocus: dialog.TransferDestSubFocusText,
		FocusField:   0, // destination path row
	}
	if kind == dialog.TransferKindCopy {
		st.PreservePermissions = a.config.Operations.PreservePermissions
		st.PreserveTimestamps = a.config.Operations.PreserveTimestamps
	}
	a.model.TransferDialog = st
	a.clearTransientMessage()
	a.armTransferDestinationValidateTimer()
}

// selectionsCommonRoot folds the parents of the active panel's selected paths into their
// deepest common ancestor. multiDir reports whether selections span more than one parent
// directory. ok is false when there are no selections or they mix schemes/hosts.
func (a *App) selectionsCommonRoot() (root pathloc.Path, multiDir bool, ok bool) {
	p := a.activePanel()
	for sel := range p.SelectedPaths {
		loc, err := pathloc.Parse(sel)
		if err != nil {
			return pathloc.Path{}, false, false
		}
		parent := loc.Parent()
		switch {
		case root.IsZero():
			root = parent
		case !parent.Equal(root):
			multiDir = true
			anc, ok := pathloc.CommonAncestor(root, parent)
			if !ok {
				// ponytail: mixed schemes/hosts have no common root; proceed as before
				return pathloc.Path{}, false, false
			}
			root = anc
		}
	}
	return root, multiDir, !root.IsZero()
}

// multiDirSelectionCommonRoot returns the deepest common ancestor of the active panel's
// selected paths when they span multiple parent directories and the panel is not already
// at that ancestor. Copy/move issued elsewhere is ambiguous; the caller shows a confirm
// offering to navigate there instead of opening the transfer dialog.
func (a *App) multiDirSelectionCommonRoot() (string, bool) {
	root, multiDir, ok := a.selectionsCommonRoot()
	if !ok || !multiDir || a.activePanel().Path.Equal(root) {
		return "", false
	}
	return root.String(), true
}

// handleAmbiguousTransferKey drives the "Ambiguous command" confirm; OK navigates the
// active panel to the selections' common root (the user re-issues copy/move from there).
func (a *App) handleAmbiguousTransferKey(event *tcell.EventKey) {
	confirm := func() {
		root := a.model.AmbiguousTransfer.CommonRoot
		a.model.AmbiguousTransfer = dialog.AmbiguousTransferState{}
		if err := a.navigatePanelToDirectory(a.model.ActivePanel, root, ""); err != nil {
			a.setErrorMessage("Navigate failed", err)
		}
	}
	cancel := func() {
		a.model.AmbiguousTransfer = dialog.AmbiguousTransferState{}
	}
	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		switch event.Rune() {
		case 'o', 'O':
			confirm()
			return
		case 'c', 'C':
			cancel()
			return
		}
	}
	switch event.Key() {
	case tcell.KeyEsc:
		cancel()
	case tcell.KeyLeft:
		a.model.AmbiguousTransfer.Focus = dialog.DialogPairLeftRight(a.model.AmbiguousTransfer.Focus, false)
	case tcell.KeyRight:
		a.model.AmbiguousTransfer.Focus = dialog.DialogPairLeftRight(a.model.AmbiguousTransfer.Focus, true)
	case tcell.KeyEnter:
		if a.model.AmbiguousTransfer.Focus == 0 {
			confirm()
		} else {
			cancel()
		}
	}
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

// openTransferDialogSelfCopyRename opens the transfer modal directly on the "new name" step (e.g. F5/F6 onto self).
func (a *App) openTransferDialogSelfCopyRename(kind dialog.TransferKind, absDestDir, sourcePath string) {
	base := filepath.Base(sourcePath)
	st := dialog.TransferDialogState{
		Open:                 true,
		Kind:                 kind,
		Phase:                dialog.TransferPhaseSelfCopyRename,
		Destination:          dialog.FileDialogField{},
		DestSubFocus:         dialog.TransferDestSubFocusText,
		SelfCopyDestDir:      absDestDir,
		SelfCopyOrigBasename: base,
		SelfCopyNewName:      transferSelfCopyNewNamePrefilled(base),
		FocusField:           0,
	}
	if kind == dialog.TransferKindCopy {
		st.PreservePermissions = a.config.Operations.PreservePermissions
		st.PreserveTimestamps = a.config.Operations.PreserveTimestamps
	}
	a.model.TransferDialog = st
	a.clearTransientMessage()
}

func (a *App) closeTransferDialog() {
	a.transferDestValidate.Invalidate()
	a.model.TransferDialog = dialog.TransferDialogState{}
}

func (a *App) handleTransferDialogKey(event *tcell.EventKey) {
	d := &a.model.TransferDialog
	if d.Phase == dialog.TransferPhaseDestination && d.Kind == dialog.TransferKindCopy {
		if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
			switch event.Rune() {
			case 'r', 'R':
				d.PreservePermissions = !d.PreservePermissions
				return
			case 't', 'T':
				d.PreserveTimestamps = !d.PreserveTimestamps
				return
			}
		}
	}
	// Alt+O = OK, Alt+C = Cancel, Alt+P = Add paused (mnemonics; must run before field edit).
	if a.tryStandardDialogActions(event, a.confirmTransfer, a.closeTransferDialog, []dialogExtraMnemonic{
		{'p', a.confirmTransferPaused},
	}) {
		return
	}
	if event.Key() == tcell.KeyEsc {
		a.closeTransferDialog()
		return
	}
	if a.tryPathPickerHostShortcut(event) {
		return
	}
	if d.FocusField == 0 && d.Phase == dialog.TransferPhaseDestination &&
		d.DestSubFocus == dialog.TransferDestSubFocusText &&
		event.Key() == tcell.KeyTab && d.Destination.CompletionSuffix != "" {
		if d.Destination.AcceptCompletion() {
			a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
			a.armTransferDestinationValidateTimer()
			return
		}
		return
	}
	if d.FocusField == 0 && d.Phase == dialog.TransferPhaseSelfCopyRename {
		if a.editTransferFieldKey(event, &d.SelfCopyNewName) {
			return
		}
	}
	if d.FocusField == 0 && d.Phase == dialog.TransferPhaseDestination {
		if d.DestSubFocus == dialog.TransferDestSubFocusPicker {
			switch event.Key() {
			case tcell.KeyLeft:
				d.DestSubFocus = dialog.TransferDestSubFocusText
				runes := []rune(d.Destination.Value)
				d.Destination.Cursor = len(runes)
				return
			case tcell.KeyEnter:
				a.openPathPickerForTransfer()
				return
			case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyDown, tcell.KeyUp:
				d.DestSubFocus = dialog.TransferDestSubFocusText
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
					d.DestSubFocus = dialog.TransferDestSubFocusPicker
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
	if focus, ok := dialog.TransferDialogMoveFocus(*d, d.FocusField, event.Key()); ok {
		prev := d.FocusField
		d.FocusField = focus
		if prev == 0 && focus != 0 {
			d.DestSubFocus = dialog.TransferDestSubFocusText
		}
		return
	}
	if event.Key() == tcell.KeyEnter {
		tf := dialog.NewTransferDialogLinearForm(dialog.TransferDialogEffectiveNumContent(*d))
		if d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0 && d.DestSubFocus == dialog.TransferDestSubFocusText {
			a.confirmTransfer()
			return
		}
		if d.Phase == dialog.TransferPhaseSelfCopyRename && d.FocusField == 0 {
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
	if d.FocusField == 0 && d.Phase != dialog.TransferPhaseSelfCopyRename {
		if a.editTransferFieldKey(event, &d.Destination) {
			a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
			a.armTransferDestinationValidateTimer()
			return
		}
	}
	if d.Phase == dialog.TransferPhaseDestination && d.Kind == dialog.TransferKindCopy && event.Key() == tcell.KeyRune && event.Modifiers() == tcell.ModNone {
		switch event.Rune() {
		case 'r', 'R':
			if d.FocusField == 1 {
				d.PreservePermissions = !d.PreservePermissions
			}
		case 't', 'T':
			if d.FocusField == 2 {
				d.PreserveTimestamps = !d.PreserveTimestamps
			}
		case ' ':
			switch d.FocusField {
			case 1:
				d.PreservePermissions = !d.PreservePermissions
			case 2:
				d.PreserveTimestamps = !d.PreserveTimestamps
			}
		}
	}
}

func (a *App) editTransferFieldKey(event *tcell.EventKey, f *dialog.FileDialogField) bool {
	return a.handleFileDialogFieldKey(event, f, func() {
		a.syncPathFieldCompletion(f, a.transferDestinationTextWidth())
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
		if d.Kind == dialog.TransferKindCopy {
			a.setTransientMessage("No files to copy", ui.MessageUrgencyWarn)
		} else {
			a.setTransientMessage("No files to move", ui.MessageUrgencyWarn)
		}
		return
	}

	if d.Phase == dialog.TransferPhaseSelfCopyRename {
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
		d.Phase = dialog.TransferPhaseSelfCopyRename
		d.SelfCopyDestDir = absDest
		base := filepath.Base(sources[0])
		d.SelfCopyOrigBasename = base
		d.SelfCopyNewName = transferSelfCopyNewNamePrefilled(base)
		d.FocusField = 0
		a.transferDestValidate.Invalidate()
		d.DestPathInvalid = false
		d.DestPathCheckPending = false
		return
	}

	var jobType jobs.Type
	switch d.Kind {
	case dialog.TransferKindCopy:
		jobType = jobs.TypeCopy
	case dialog.TransferKindMove:
		jobType = jobs.TypeMove
	default:
		a.closeTransferDialog()
		return
	}
	sourcesCopy := append([]string(nil), sources...)
	a.activePanel().ClearSelection()
	preserve := jobs.TransferPreserve{
		PreservePermissions: d.PreservePermissions,
		PreserveTimestamps:  d.PreserveTimestamps,
	}
	a.addTransferJob(jobType, sourcesCopy, dest, startPaused, preserve)
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
	case dialog.TransferKindCopy:
		jobType = jobs.TypeCopy
	case dialog.TransferKindMove:
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
	preserve := jobs.TransferPreserve{
		PreservePermissions: d.PreservePermissions,
		PreserveTimestamps:  d.PreserveTimestamps,
	}
	a.addTransferJob(jobType, sourcesCopy, finalDest, startPaused, preserve)
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

// activePanelSources returns file paths from the active panel: selected entries
// if any, otherwise the cursor entry. Returns nil if no valid sources exist.
func (a *App) activePanelSources() []string {
	source, err := ops.ResolveSource(a.activePanel())
	if err != nil {
		return nil
	}
	return ops.SourcePaths(source)
}
