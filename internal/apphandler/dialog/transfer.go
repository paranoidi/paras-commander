package dialog

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
// With no selection, a single cursor item already shown in the passive panel opens Rename instead.
func (h *Handler) ActivateMoveAction() {
	p := h.host.ActivePanel()
	if len(p.SelectedPaths) == 0 {
		if entry, ok := p.CurrentEntry(); ok {
			dest := h.host.InactivePanel().PathString()
			if entry.Path == filepath.Join(dest, entry.Name) {
				h.OpenRenameDialog(p)
				return
			}
		}
	}
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
		Open:         true,
		Kind:         kind,
		Destination:  TransferPrefilledDestination(passive.PathString()),
		DestSubFocus: dialog.TransferDestSubFocusText,
		FocusField:   0, // destination path row
	}
	if kind == dialog.TransferKindCopy {
		cfg := h.host.Config()
		st.PreservePermissions = cfg.Operations.PreservePermissions
		st.PreserveTimestamps = cfg.Operations.PreserveTimestamps
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
	root, multiDir, ok := h.host.ActivePanel().SelectionsCommonRoot()
	if !ok || !multiDir {
		return "", false
	}
	return root.String(), true
}

// TransferPreviewListViewportRows returns how many preview-list rows the multi-location
// transfer dialog currently shows, for clamping EntriesScroll.
func (h *Handler) TransferPreviewListViewportRows() int {
	w, ht := h.screen.Size()
	return dialog.TransferListViewportRows(dialog.Layout{Width: w, Height: ht}, h.model.TransferDialog)
}

// scrollTransferPreviewList moves the multi-location preview list scroll by delta rows,
// clamped to the current viewport.
func (h *Handler) scrollTransferPreviewList(delta int) {
	st := &h.model.TransferDialog
	vp := h.TransferPreviewListViewportRows()
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

// OpenTransferDialogSelfCopyRename opens the transfer modal directly on the "new name" step
// (e.g. F5/F6 onto self).
func (h *Handler) OpenTransferDialogSelfCopyRename(kind dialog.TransferKind, absDestDir, sourcePath string) {
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
		cfg := h.host.Config()
		st.PreservePermissions = cfg.Operations.PreservePermissions
		st.PreserveTimestamps = cfg.Operations.PreserveTimestamps
	}
	h.model.TransferDialog = st
	h.host.ClearTransientMessage()
}

// CloseTransferDialog closes the unified transfer (copy/move) dialog and invalidates its
// debounced destination-path validation.
func (h *Handler) CloseTransferDialog() {
	h.transferDestValidate.Invalidate()
	h.model.TransferDialog = dialog.TransferDialogState{}
	h.model.DestinationTargetPrimary = false
	h.model.DestinationTargetSecondary = false
}

// handleTransferAltShortcut handles the Alt-letter mnemonics that must run before standard
// dialog actions and field editing: Alt+R/Alt+T toggle preserve-permissions/timestamps on a
// copy's destination phase, and Alt+I toggles "Flatten into destination" for a multi-location
// transfer. Returns true when the rune was handled.
func (h *Handler) handleTransferAltShortcut(event *tcell.EventKey) bool {
	d := &h.model.TransferDialog
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
func (h *Handler) handleTransferDestinationNav(event *tcell.EventKey) bool {
	d := &h.model.TransferDialog
	if d.Phase != dialog.TransferPhaseDestination {
		return false
	}
	return h.DestFieldNav(event, &d.Destination, &d.DestSubFocus, &d.FocusField,
		dialog.TransferDestSubFocusText, dialog.TransferDestSubFocusPicker, h.OpenPathPickerForTransfer)
}

// handleTransferEnter handles Enter on the transfer dialog: confirm from the destination
// text field or self-copy rename field, or activate the focused Cancel/OK/Add-paused
// button. Returns true when handled (caller should return); false when the key isn't Enter
// or Enter didn't land on a recognized target, so the caller falls through to field editing.
func (h *Handler) handleTransferEnter(event *tcell.EventKey) bool {
	d := &h.model.TransferDialog
	if event.Key() != tcell.KeyEnter {
		return false
	}
	tf := dialog.NewTransferDialogLinearForm(dialog.TransferDialogEffectiveNumContent(*d))
	if d.Phase == dialog.TransferPhaseDestination && d.FocusField == 0 && d.DestSubFocus == dialog.TransferDestSubFocusText {
		h.confirmTransfer()
		return true
	}
	if d.Phase == dialog.TransferPhaseSelfCopyRename && d.FocusField == 0 {
		h.confirmTransfer()
		return true
	}
	switch d.FocusField {
	case tf.CancelIndex():
		h.CloseTransferDialog()
		return true
	case tf.OKIndex():
		h.confirmTransfer()
		return true
	case tf.AddPausedIndex():
		h.confirmTransferPaused()
		return true
	}
	return false
}

// handleTransferCheckboxRune applies rune shortcuts (mnemonic letters and Space) for the
// permissions/timestamps checkboxes and the multi-location flatten checkbox when they are
// focused.
func (h *Handler) handleTransferCheckboxRune(event *tcell.EventKey) {
	d := &h.model.TransferDialog
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

// HandleTransferDialogKey routes a key event for the open unified transfer (copy/move) dialog.
func (h *Handler) HandleTransferDialogKey(event *tcell.EventKey) {
	d := &h.model.TransferDialog
	if h.handleTransferAltShortcut(event) {
		return
	}
	// Alt+O = OK, Alt+C = Cancel, Alt+P = Add paused (mnemonics; must run before field edit).
	if dialog.TryStandardDialogActions(event, h.confirmTransfer, h.CloseTransferDialog, []dialog.ExtraMnemonic{
		{Rune: 'p', Fn: h.confirmTransferPaused},
	}) {
		return
	}
	if event.Key() == tcell.KeyEsc {
		h.CloseTransferDialog()
		return
	}
	if d.MultiLocation() {
		switch event.Key() {
		case tcell.KeyPgUp:
			h.scrollTransferPreviewList(-h.TransferPreviewListViewportRows())
			return
		case tcell.KeyPgDn:
			h.scrollTransferPreviewList(h.TransferPreviewListViewportRows())
			return
		}
	}
	if h.TryPathPickerHostShortcut(event) {
		return
	}
	if h.TryTransferDialogDestinationShortcut(event) {
		return
	}
	if d.Phase == dialog.TransferPhaseDestination && event.Key() == tcell.KeyTab &&
		h.DestFieldAcceptCompletion(&d.Destination, d.DestSubFocus, d.FocusField, dialog.TransferDestSubFocusText, h.ArmTransferDestinationValidateTimer) {
		return
	}
	if d.FocusField == 0 && d.Phase == dialog.TransferPhaseSelfCopyRename {
		if h.editTransferFieldKey(event, &d.SelfCopyNewName) {
			return
		}
	}
	if h.handleTransferDestinationNav(event) {
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
	if h.handleTransferEnter(event) {
		return
	}
	if d.FocusField == 0 && d.Phase != dialog.TransferPhaseSelfCopyRename {
		if h.editTransferFieldKey(event, &d.Destination) {
			h.SyncPathFieldCompletion(&d.Destination, h.TransferDestinationTextWidth())
			h.ArmTransferDestinationValidateTimer()
			return
		}
	}
	h.handleTransferCheckboxRune(event)
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
	absDest := destLoc.String()

	srcLocs := make([]pathloc.Path, len(sources))
	for i, src := range sources {
		srcLocs[i] = pathloc.MustParse(src)
	}
	flat := d.MultiLocation() && d.FlattenIntoDest
	nSelf := ops.SelfTargetCount(srcLocs, destLoc, flat)
	if nSelf > 0 {
		if len(sources) > 1 {
			h.host.SetTransientMessage("Cannot transfer multiple items when some would overwrite themselves", ui.MessageUrgencyWarn)
			return
		}
		d.Phase = dialog.TransferPhaseSelfCopyRename
		d.SelfCopyDestDir = absDest
		base := filepath.Base(sources[0])
		d.SelfCopyOrigBasename = base
		d.SelfCopyNewName = transferSelfCopyNewNamePrefilled(base)
		d.FocusField = 0
		h.transferDestValidate.Invalidate()
		d.DestPathInvalid = false
		d.DestPathCheckPending = false
		h.model.DestinationTargetPrimary = false
		h.model.DestinationTargetSecondary = false
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
	sourcesCopy := append([]string(nil), sources...)
	h.host.ActivePanel().ClearSelection()
	preserve := jobs.TransferPreserve{
		PreservePermissions: d.PreservePermissions,
		PreserveTimestamps:  d.PreserveTimestamps,
		FlattenIntoDest:     flat,
	}
	h.AddTransferJob(jobType, sourcesCopy, dest, startPaused, preserve)
	h.CloseTransferDialog()
	h.setTransferQueuedMessage(jobType, startPaused)
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
	h.host.ActivePanel().ClearSelection()
	preserve := jobs.TransferPreserve{
		PreservePermissions: d.PreservePermissions,
		PreserveTimestamps:  d.PreserveTimestamps,
	}
	h.AddTransferJob(jobType, sourcesCopy, finalDest, startPaused, preserve)
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
