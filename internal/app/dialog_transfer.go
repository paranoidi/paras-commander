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
	if root, ok := a.multiDirSelectionCommonRoot(); ok {
		st.CommonRoot = root
		st.Entries = a.transferPreviewEntries(root)
	}
	a.model.TransferDialog = st
	a.clearTransientMessage()
	a.armTransferDestinationValidateTimer()
}

// transferPreviewEntries builds the multi-location preview list: the active panel's
// selection resolved and labeled relative to root (not the panel's current path), so
// the preview reflects where the transfer will read from.
func (a *App) transferPreviewEntries(root string) []dialog.DeleteListEntry {
	source, err := ops.ResolveSource(a.activePanel())
	if err != nil {
		return nil
	}
	homeDir := a.model.UserHomeDir
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

// selectionsCommonRoot delegates to panel.State.SelectionsCommonRoot for the active panel.
func (a *App) selectionsCommonRoot() (root pathloc.Path, multiDir bool, ok bool) {
	return a.activePanel().SelectionsCommonRoot()
}

// multiDirSelectionCommonRoot returns the deepest common ancestor of the active panel's
// selected paths when they span multiple parent directories, regardless of the panel's
// current path. The transfer dialog always shows the multi-location Source/Result preview
// in that case.
func (a *App) multiDirSelectionCommonRoot() (string, bool) {
	root, multiDir, ok := a.selectionsCommonRoot()
	if !ok || !multiDir {
		return "", false
	}
	return root.String(), true
}

// transferPreviewListViewportRows returns how many preview-list rows the multi-location
// transfer dialog currently shows, for clamping EntriesScroll.
func (a *App) transferPreviewListViewportRows() int {
	w, h := a.screen.Size()
	return dialog.TransferListViewportRows(dialog.Layout{Width: w, Height: h}, a.model.TransferDialog)
}

// scrollTransferPreviewList moves the multi-location preview list scroll by delta rows,
// clamped to the current viewport.
func (a *App) scrollTransferPreviewList(delta int) {
	st := &a.model.TransferDialog
	vp := a.transferPreviewListViewportRows()
	maxScroll := len(st.Entries) - vp
	if maxScroll < 0 {
		maxScroll = 0
	}
	st.EntriesScroll += delta
	if st.EntriesScroll < 0 {
		st.EntriesScroll = 0
	}
	if st.EntriesScroll > maxScroll {
		st.EntriesScroll = maxScroll
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
	a.model.DestinationTargetPrimary = false
	a.model.DestinationTargetSecondary = false
}

// handleTransferAltShortcut handles the Alt-letter mnemonics that must run before standard
// dialog actions and field editing: Alt+R/Alt+T toggle preserve-permissions/timestamps on a
// copy's destination phase, and Alt+I toggles "Flatten into destination" for a multi-location
// transfer. Returns true when the rune was handled.
func (a *App) handleTransferAltShortcut(event *tcell.EventKey) bool {
	d := &a.model.TransferDialog
	if d.Phase == dialog.TransferPhaseDestination && d.Kind == dialog.TransferKindCopy {
		if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
			switch event.Rune() {
			case 'r', 'R':
				d.PreservePermissions = !d.PreservePermissions
				return true
			case 't', 'T':
				d.PreserveTimestamps = !d.PreserveTimestamps
				return true
			}
		}
	}
	// Alt+I toggles "Flatten into destination"; applies to both copy and move when
	// the transfer is issued on a selection spanning multiple directories (MultiLocation).
	if d.MultiLocation() {
		if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
			switch event.Rune() {
			case 'i', 'I':
				d.FlattenIntoDest = !d.FlattenIntoDest
				return true
			}
		}
	}
	return false
}

// handleTransferDestinationNav handles Left/Right cursor movement and picker sub-focus
// navigation on the destination field while it is focused. Returns true when the key was
// handled (caller should return); false to fall through to the generic focus-move / Enter
// handling below.
func (a *App) handleTransferDestinationNav(event *tcell.EventKey) bool {
	d := &a.model.TransferDialog
	if d.Phase != dialog.TransferPhaseDestination {
		return false
	}
	return a.destFieldNav(event, &d.Destination, &d.DestSubFocus, &d.FocusField,
		dialog.TransferDestSubFocusText, dialog.TransferDestSubFocusPicker, a.openPathPickerForTransfer)
}

// handleTransferEnter handles Enter on the transfer dialog: confirm from the destination
// text field or self-copy rename field, or activate the focused Cancel/OK/Add-paused
// button. Returns true when handled (caller should return); false when the key isn't Enter
// or Enter didn't land on a recognized target, so the caller falls through to field editing.
func (a *App) handleTransferEnter(event *tcell.EventKey) bool {
	d := &a.model.TransferDialog
	if event.Key() != tcell.KeyEnter {
		return false
	}
	tf := dialog.NewTransferDialogLinearForm(dialog.TransferDialogEffectiveNumContent(*d))
	if d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0 && d.DestSubFocus == dialog.TransferDestSubFocusText {
		a.confirmTransfer()
		return true
	}
	if d.Phase == dialog.TransferPhaseSelfCopyRename && d.FocusField == 0 {
		a.confirmTransfer()
		return true
	}
	switch d.FocusField {
	case tf.CancelIndex():
		a.closeTransferDialog()
		return true
	case tf.OKIndex():
		a.confirmTransfer()
		return true
	case tf.AddPausedIndex():
		a.confirmTransferPaused()
		return true
	}
	return false
}

// handleTransferCheckboxRune applies rune shortcuts (mnemonic letters and Space) for the
// permissions/timestamps checkboxes and the multi-location flatten checkbox when they are
// focused.
func (a *App) handleTransferCheckboxRune(event *tcell.EventKey) {
	d := &a.model.TransferDialog
	if d.Phase != dialog.TransferPhaseDestination || event.Key() != tcell.KeyRune || event.Modifiers() != tcell.ModNone {
		return
	}
	if d.Kind == dialog.TransferKindCopy {
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
	if d.MultiLocation() {
		flattenIdx := dialog.TransferDialogEffectiveNumContent(*d) - 1
		if d.FocusField == flattenIdx {
			switch event.Rune() {
			case 'i', 'I', ' ':
				d.FlattenIntoDest = !d.FlattenIntoDest
			}
		}
	}
}

func (a *App) handleTransferDialogKey(event *tcell.EventKey) {
	d := &a.model.TransferDialog
	if a.handleTransferAltShortcut(event) {
		return
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
	if d.MultiLocation() {
		switch event.Key() {
		case tcell.KeyPgUp:
			a.scrollTransferPreviewList(-a.transferPreviewListViewportRows())
			return
		case tcell.KeyPgDn:
			a.scrollTransferPreviewList(a.transferPreviewListViewportRows())
			return
		}
	}
	if a.tryPathPickerHostShortcut(event) {
		return
	}
	if a.tryTransferDialogDestinationShortcut(event) {
		return
	}
	if d.Phase == dialog.TransferPhaseDestination && event.Key() == tcell.KeyTab &&
		a.destFieldAcceptCompletion(&d.Destination, d.DestSubFocus, d.FocusField, dialog.TransferDestSubFocusText, a.armTransferDestinationValidateTimer) {
		return
	}
	if d.FocusField == 0 && d.Phase == dialog.TransferPhaseSelfCopyRename {
		if a.editTransferFieldKey(event, &d.SelfCopyNewName) {
			return
		}
	}
	if a.handleTransferDestinationNav(event) {
		return
	}
	if focus, ok := dialog.TransferDialogMoveFocus(*d, d.FocusField, event.Key()); ok {
		prev := d.FocusField
		d.FocusField = focus
		if prev == 0 && focus != 0 {
			d.DestSubFocus = dialog.TransferDestSubFocusText
		}
		return
	}
	if a.handleTransferEnter(event) {
		return
	}
	if d.FocusField == 0 && d.Phase != dialog.TransferPhaseSelfCopyRename {
		if a.editTransferFieldKey(event, &d.Destination) {
			a.syncPathFieldCompletion(&d.Destination, a.transferDestinationTextWidth())
			a.armTransferDestinationValidateTimer()
			return
		}
	}
	a.handleTransferCheckboxRune(event)
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

	srcLocs := make([]pathloc.Path, len(sources))
	for i, src := range sources {
		srcLocs[i] = pathloc.MustParse(src)
	}
	flat := d.MultiLocation() && d.FlattenIntoDest
	nSelf := ops.SelfTargetCount(srcLocs, destLoc, flat)
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
		a.model.DestinationTargetPrimary = false
		a.model.DestinationTargetSecondary = false
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
		FlattenIntoDest:     flat,
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
