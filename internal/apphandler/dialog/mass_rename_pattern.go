package dialog

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// maxMassRenameHistory caps the in-memory, session-only recently-used pattern list (see
// Handler.massRenameHistory), mirroring panel.State.recordVisit's dedup-prepend-cap shape.
const maxMassRenameHistory = 20

// massRenamePatternsPath resolves the configured patterns.toml path.
func (h *Handler) massRenamePatternsPath() string {
	return ops.MassRenamePatternsResolveFile(h.host.Config().MassRename.File, h.configDir)
}

func massRenameModeString(m dialog.MassRenameModeUI) string {
	switch m {
	case dialog.MassRenameModeUIRegex:
		return "regex"
	case dialog.MassRenameModeUICapitalize:
		return "capitalize"
	default:
		return "simple"
	}
}

func massRenameModeFromString(s string) dialog.MassRenameModeUI {
	switch s {
	case "regex":
		return dialog.MassRenameModeUIRegex
	case "capitalize":
		return dialog.MassRenameModeUICapitalize
	default:
		return dialog.MassRenameModeUISimple
	}
}

// MassRenameSavePatternFooterEligible reports whether the footer should show the F5 "Save
// pattern" hint: the mass-rename dialog is open on its main screen with a mode that has
// something meaningful to save (not External $EDITOR, which has no Find/Replace text).
func (h *Handler) MassRenameSavePatternFooterEligible() bool {
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMassRename || d.MassRenamePhase != dialog.MassRenamePhaseMain {
		return false
	}
	return d.MassRenameMode != dialog.MassRenameModeUIExternalEditor
}

// MassRenameOverwritePatternFooterEligible reports whether the footer should show the F5
// "Overwrite existing" hint: the save-pattern prompt is open, where F5 (same binding as "Save
// pattern" on the main screen) opens the overwrite picker instead.
func (h *Handler) MassRenameOverwritePatternFooterEligible() bool {
	d := &h.model.FileDialog
	return d.Open && d.DialogType == dialog.FileDialogMassRename && d.MassRenamePhase == dialog.MassRenamePhaseSavePrompt
}

// MassRenameLoadPatternFooterEligible reports whether the footer should show the F2 "Load
// pattern" hint: the mass-rename dialog is open on its main screen.
func (h *Handler) MassRenameLoadPatternFooterEligible() bool {
	d := &h.model.FileDialog
	return d.Open && d.DialogType == dialog.FileDialogMassRename && d.MassRenamePhase == dialog.MassRenamePhaseMain
}

// MassRenameHistoryFooterEligible reports whether the footer should show the F3 "History" hint:
// the mass-rename dialog is open on its main screen.
func (h *Handler) MassRenameHistoryFooterEligible() bool {
	d := &h.model.FileDialog
	return d.Open && d.DialogType == dialog.FileDialogMassRename && d.MassRenamePhase == dialog.MassRenamePhaseMain
}

// massRenamePickerPhaseOpen reports whether phase is one of the picker sub-screens (load,
// history or overwrite) that share handleMassRenamePickerKey's key handling and
// currentMassRenamePickerState's backing-state selection.
func massRenamePickerPhaseOpen(phase dialog.MassRenamePhase) bool {
	return dialog.MassRenamePickerPhase(phase)
}

// MassRenameDeletePatternFooterEligible reports whether the footer should show the F8 "Delete
// pattern" hint: the load-pattern or pattern-history picker is open with a ranked entry selected.
func (h *Handler) MassRenameDeletePatternFooterEligible() bool {
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMassRename || !massRenamePickerPhaseOpen(d.MassRenamePhase) {
		return false
	}
	st := h.currentMassRenamePickerState()
	return len(st.Ranked) > 0 && st.Selected >= 0 && st.Selected < len(st.Ranked)
}

// currentMassRenamePickerState returns a pointer to whichever picker state matches the
// currently open mass-rename sub-phase (Load or History); both share the same widget shape
// (MassRenamePatternPickerState) and key handling, differing only in where entries come from and
// how F8 deletes one.
func (h *Handler) currentMassRenamePickerState() *dialog.MassRenamePatternPickerState {
	d := &h.model.FileDialog
	if d.MassRenamePhase == dialog.MassRenamePhaseHistoryPicker {
		return &d.MassRenameHistoryPicker
	}
	return &d.MassRenameLoadPicker
}

// tryMassRenameDialogShortcut handles [dialog.mass_rename] while the mass-rename dialog's
// main screen is active. Returns true when the event was consumed.
func (h *Handler) tryMassRenameDialogShortcut(ev *tcell.EventKey) bool {
	if h.keysMassRenameDialog == nil {
		return false
	}
	d := &h.model.FileDialog
	if !d.Open || d.DialogType != dialog.FileDialogMassRename || d.MassRenamePhase != dialog.MassRenamePhaseMain {
		return false
	}
	id, ok := h.keysMassRenameDialog.Lookup(ev)
	if !ok {
		return false
	}
	switch id {
	case keymap.ActionFileMassRenameSavePattern:
		if d.MassRenameMode == dialog.MassRenameModeUIExternalEditor {
			return false
		}
		h.openMassRenameSavePrompt()
		return true
	case keymap.ActionFileMassRenameLoadPattern:
		h.openMassRenameLoadPicker()
		return true
	case keymap.ActionFileMassRenameHistory:
		h.openMassRenameHistoryPicker()
		return true
	default:
		return false
	}
}

// openMassRenameSavePrompt swaps in the Name/Description save-pattern sub-screen, stashing
// the main dialog's Find/Replace fields to restore on cancel or confirm.
func (h *Handler) openMassRenameSavePrompt() {
	d := &h.model.FileDialog
	d.MassRenameSavedFields = d.Fields
	d.Fields = []dialog.FileDialogField{
		{Label: "Name"},
		{Label: "Description"},
	}
	d.MassRenamePhase = dialog.MassRenamePhaseSavePrompt
	d.FocusedField = 0
}

// closeMassRenameSavePrompt restores the main dialog's fields and returns to the main screen
// (Esc / Cancel).
func (h *Handler) closeMassRenameSavePrompt() {
	h.restoreMassRenameMainScreen()
}

// confirmMassRenameSavePrompt validates the Name field, upserts the pattern into patterns.toml
// (overwriting any existing entry with the same name), and returns to the main screen (OK).
func (h *Handler) confirmMassRenameSavePrompt() {
	d := &h.model.FileDialog
	if len(d.Fields) < 2 {
		h.closeMassRenameSavePrompt()
		return
	}
	name := strings.TrimSpace(d.Fields[0].Value)
	if name == "" {
		h.host.SetTransientMessage("Name is required", ui.MessageUrgencyWarn)
		return
	}
	description := strings.TrimSpace(d.Fields[1].Value)

	find, replace := h.massRenameStashedFindReplace()
	p := h.massRenameCurrentPattern(name, description, find, replace)
	if err := ops.UpsertMassRenamePattern(h.massRenamePatternsPath(), p); err != nil {
		h.host.SetErrorMessage("Save pattern", err)
		return
	}
	h.massRenamePatternName = name
	h.restoreMassRenameMainScreen()
	h.host.SetTransientMessage(fmt.Sprintf("Pattern saved: %s", name), ui.MessageUrgencyInfo)
}

// restoreMassRenameMainScreen unstashes the main find/replace fields saved by
// openMassRenameSavePrompt and returns to the main screen. Shared by the save prompt's OK and
// by the overwrite picker, which both end the save flow.
func (h *Handler) restoreMassRenameMainScreen() {
	d := &h.model.FileDialog
	d.Fields = d.MassRenameSavedFields
	d.MassRenameSavedFields = nil
	d.MassRenamePhase = dialog.MassRenamePhaseMain
	d.FocusedField = dialog.MassRenameFindFieldFocus
}

// handleMassRenameSavePromptKey routes keys while the save-pattern prompt is open. Mirrors
// handleRenameToolKey's shape: focus cycles Name(0) -> Description(1) -> OK(okIdx) ->
// Cancel(cancelIdx); Left/Right move buttons (or the cursor within a focused field);
// Up/Down/Tab/Backtab move between content rows and stop at the button row (Left/Right then
// moves between OK and Cancel), per the project's dialog navigation standard.
func (h *Handler) handleMassRenameSavePromptKey(event *tcell.EventKey) bool {
	d := &h.model.FileDialog
	okIdx := len(d.Fields)
	cancelIdx := okIdx + 1

	if h.keysMassRenameDialog != nil {
		if id, ok := h.keysMassRenameDialog.Lookup(event); ok && id == keymap.ActionFileMassRenameSavePattern {
			h.openMassRenameOverwritePicker()
			return false
		}
	}

	if event.Key() == tcell.KeyRune && keymap.AltLetterModifiers(event.Modifiers()) {
		if dialog.TryStandardDialogActions(event, h.confirmMassRenameSavePrompt, h.closeMassRenameSavePrompt, nil) {
			return false
		}
	}

	fieldAt := func(idx int) *dialog.FileDialogField {
		if idx < 0 || idx >= len(d.Fields) {
			return nil
		}
		return &d.Fields[idx]
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.closeMassRenameSavePrompt()
		return false
	case tcell.KeyEnter:
		if d.FocusedField == cancelIdx {
			h.closeMassRenameSavePrompt()
		} else {
			h.confirmMassRenameSavePrompt()
		}
		return false
	case tcell.KeyDown, tcell.KeyTab:
		if d.FocusedField < okIdx {
			d.FocusedField++
		}
		return false
	case tcell.KeyUp, tcell.KeyBacktab:
		switch {
		case d.FocusedField == okIdx || d.FocusedField == cancelIdx:
			d.FocusedField = okIdx - 1
		case d.FocusedField > 0:
			d.FocusedField--
		}
		return false
	case tcell.KeyLeft:
		switch {
		case d.FocusedField == cancelIdx:
			d.FocusedField = okIdx
		case d.FocusedField < okIdx:
			dialog.HandleFileDialogFieldKey(event, fieldAt(d.FocusedField), h.keysDialogInput, nil)
		}
		return false
	case tcell.KeyRight:
		switch {
		case d.FocusedField == okIdx:
			d.FocusedField = cancelIdx
		case d.FocusedField < okIdx:
			dialog.HandleFileDialogFieldKey(event, fieldAt(d.FocusedField), h.keysDialogInput, nil)
		}
		return false
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyBackspace, tcell.KeyBackspace2, tcell.KeyDelete, tcell.KeyCtrlL:
		if d.FocusedField < okIdx {
			dialog.HandleFileDialogFieldKey(event, fieldAt(d.FocusedField), h.keysDialogInput, nil)
		}
		return false
	case tcell.KeyRune:
		if d.FocusedField < okIdx {
			dialog.HandleFileDialogFieldKey(event, fieldAt(d.FocusedField), h.keysDialogInput, nil)
			return false
		}
		if keymap.IsPlainPrintableRune(event) && event.Rune() == ' ' {
			if d.FocusedField == cancelIdx {
				h.closeMassRenameSavePrompt()
			} else {
				h.confirmMassRenameSavePrompt()
			}
		}
		return false
	}
	return false
}

// massRenameStashedFindReplace returns the main screen's Find/Replace values stashed in
// MassRenameSavedFields while the save-pattern prompt owns d.Fields.
func (h *Handler) massRenameStashedFindReplace() (find, replace string) {
	fields := h.model.FileDialog.MassRenameSavedFields
	if len(fields) > 0 {
		find = fields[0].Value
	}
	if len(fields) > 1 {
		replace = fields[1].Value
	}
	return find, replace
}

// massRenameCurrentPattern builds an ops.MassRenamePattern from the given name/description/
// find/replace and the dialog's current mode and option flags. Shared by
// confirmMassRenameSavePrompt (name/description from the save prompt) and history recording
// (name/description left blank).
func (h *Handler) massRenameCurrentPattern(name, description, find, replace string) ops.MassRenamePattern {
	d := &h.model.FileDialog
	return ops.MassRenamePattern{
		Name:        name,
		Description: description,
		Mode:        massRenameModeString(d.MassRenameMode),
		Find:        find,
		Replace:     replace,
		CaseFold:    d.MassRenameCaseFold,
		StripSpaces: d.MassRenameStripSpaces,
		CapEachWord: d.MassRenameCapEachWord,
		CapPunctSep: d.MassRenameCapPunctSep,
	}
}

// recordMassRenameHistory prepends p to the in-memory pattern history, removing any existing
// equal entry first and capping the list at maxMassRenameHistory (mirrors
// panel.State.recordVisit's remove-then-prepend-then-cap shape). p.Name/p.Description are
// expected to be blank (history entries are unnamed; see massRenameCurrentPattern's callers).
func (h *Handler) recordMassRenameHistory(p ops.MassRenamePattern) {
	hist := make([]ops.MassRenamePattern, 0, len(h.massRenameHistory)+1)
	hist = append(hist, p)
	for _, e := range h.massRenameHistory {
		if e == p {
			continue
		}
		hist = append(hist, e)
	}
	if len(hist) > maxMassRenameHistory {
		hist = hist[:maxMassRenameHistory]
	}
	h.massRenameHistory = hist
}

// removeMassRenameHistoryEntry splices a matching entry out of the in-memory pattern history
// (F8 on the pattern-history picker).
func (h *Handler) removeMassRenameHistoryEntry(p ops.MassRenamePattern) {
	out := h.massRenameHistory[:0:0]
	for _, e := range h.massRenameHistory {
		if e == p {
			continue
		}
		out = append(out, e)
	}
	h.massRenameHistory = out
}

// openMassRenameLoadPicker opens the fuzzy load-pattern picker, lazily loading patterns.toml (no
// caching, mirroring the bookmarks / meta dialogs).
func (h *Handler) openMassRenameLoadPicker() {
	d := &h.model.FileDialog
	items, err := ops.LoadMassRenamePatterns(h.massRenamePatternsPath())
	if err != nil {
		h.host.SetErrorMessage("Load pattern", err)
		return
	}
	d.MassRenameLoadPicker = dialog.MassRenamePatternPickerState{Items: items}
	d.MassRenamePhase = dialog.MassRenamePhaseLoadPicker
	h.syncMassRenamePickerRanks()
	h.selectMassRenamePatternByName(&d.MassRenameLoadPicker, h.massRenamePatternName)
}

// selectMassRenamePatternByName moves the picker cursor onto the entry named name, so the load
// and overwrite pickers open on the pattern last loaded, saved or overwritten this session
// (see Handler.massRenamePatternName). No-op when name is empty or no longer in the list.
func (h *Handler) selectMassRenamePatternByName(st *dialog.MassRenamePatternPickerState, name string) {
	if name == "" {
		return
	}
	for i, entIdx := range st.Ranked {
		if entIdx < 0 || entIdx >= len(st.Items) || st.Items[entIdx].Name != name {
			continue
		}
		st.Selected = i
		dialog.EnsureMassRenamePatternPickerListScroll(st, h.MassRenamePatternPickerListRows())
		return
	}
}

// openMassRenameOverwritePicker opens the saved-patterns picker from the save-pattern prompt
// (F5): picking an entry writes the current find/replace over it. Shares
// MassRenameLoadPicker's state and rendering with the load picker; only
// activateMassRenamePickerSelection and closeMassRenamePicker branch on the phase.
func (h *Handler) openMassRenameOverwritePicker() {
	d := &h.model.FileDialog
	items, err := ops.LoadMassRenamePatterns(h.massRenamePatternsPath())
	if err != nil {
		h.host.SetErrorMessage("Overwrite pattern", err)
		return
	}
	if len(items) == 0 {
		h.host.SetTransientMessage("No saved patterns", ui.MessageUrgencyWarn)
		return
	}
	d.MassRenameLoadPicker = dialog.MassRenamePatternPickerState{Items: items}
	d.MassRenamePhase = dialog.MassRenamePhaseOverwritePicker
	h.syncMassRenamePickerRanks()
	h.selectMassRenamePatternByName(&d.MassRenameLoadPicker, h.massRenamePatternName)
}

// overwriteMassRenamePattern replaces the picked saved pattern with the current mode/find/
// replace/options, keeping its name and (unless the save prompt's Description field was typed
// into) its description, then ends the save flow on the main screen.
func (h *Handler) overwriteMassRenamePattern(target ops.MassRenamePattern) {
	d := &h.model.FileDialog
	description := target.Description
	if len(d.Fields) > 1 {
		if typed := strings.TrimSpace(d.Fields[1].Value); typed != "" {
			description = typed
		}
	}
	find, replace := h.massRenameStashedFindReplace()
	p := h.massRenameCurrentPattern(target.Name, description, find, replace)
	if err := ops.UpsertMassRenamePattern(h.massRenamePatternsPath(), p); err != nil {
		h.host.SetErrorMessage("Overwrite pattern", err)
		return
	}
	h.massRenamePatternName = target.Name
	h.restoreMassRenameMainScreen()
	h.host.SetTransientMessage(fmt.Sprintf("Pattern overwritten: %s", target.Name), ui.MessageUrgencyInfo)
}

// openMassRenameHistoryPicker opens the fuzzy pattern-history picker over the in-memory,
// session-only recently-used pattern list (see Handler.massRenameHistory). Items is a snapshot
// copy so splicing it on delete never mutates massRenameHistory's own backing array directly;
// removeMassRenameHistoryEntry is the single source of truth for that.
func (h *Handler) openMassRenameHistoryPicker() {
	d := &h.model.FileDialog
	d.MassRenameHistoryPicker = dialog.MassRenamePatternPickerState{Items: slices.Clone(h.massRenameHistory)}
	d.MassRenamePhase = dialog.MassRenamePhaseHistoryPicker
	h.syncMassRenamePickerRanks()
}

// closeMassRenamePicker leaves the open picker without applying a selection (Esc / Cancel).
// The overwrite picker returns to the save-pattern prompt it was opened from; the load and
// history pickers return to the main screen. d.Fields are left untouched either way.
func (h *Handler) closeMassRenamePicker() {
	d := &h.model.FileDialog
	if d.MassRenamePhase == dialog.MassRenamePhaseOverwritePicker {
		d.MassRenamePhase = dialog.MassRenamePhaseSavePrompt
		d.FocusedField = 0
		return
	}
	d.MassRenamePhase = dialog.MassRenamePhaseMain
	d.FocusedField = dialog.MassRenameFindFieldFocus
}

// syncMassRenamePickerRanks re-filters and re-ranks the open picker's item list (load or
// history) against the current query, clamping selection and list scroll.
func (h *Handler) syncMassRenamePickerRanks() {
	st := h.currentMassRenamePickerState()
	lines := make([]string, len(st.Items))
	for i, p := range st.Items {
		lines[i] = dialog.MassRenamePatternSearchLine(p)
	}
	cfg := h.host.Config()
	st.Ranked, st.MatchRanges = h.host.SyncFilteredListRanks(lines, st.Query, len(st.Items), cfg.Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&st.Selected, len(st.Ranked))
	dialog.EnsureMassRenamePatternPickerListScroll(st, h.MassRenamePatternPickerListRows())
}

// MassRenamePatternPickerListRows returns how many rows the open picker's fuzzy list (load or
// history) currently shows, derived from the dialog's actual on-screen rect (see
// massRenamePatternPickerDialogHeight / drawMassRenamePatternPickerContent).
func (h *Handler) MassRenamePatternPickerListRows() int {
	rows := h.FileDialogRect().Height - 6
	if rows < 1 {
		rows = 1
	}
	return rows
}

// MassRenamePatternPickerQueryWidth returns the visible width of the open picker's query input
// row.
func (h *Handler) MassRenamePatternPickerQueryWidth() int {
	w := h.FileDialogRect().Width - 4
	if w < 10 {
		w = 10
	}
	return w
}

// activateMassRenamePickerSelection applies the selected pattern (Enter / OK). On the load and
// history pickers it loads the pattern back into the main mass-rename dialog's mode/fields/
// options and recomputes the preview; on the overwrite picker it instead writes the current
// settings over the picked saved pattern (see overwriteMassRenamePattern).
func (h *Handler) activateMassRenamePickerSelection() {
	d := &h.model.FileDialog
	st := h.currentMassRenamePickerState()
	if len(st.Ranked) == 0 || st.Selected < 0 || st.Selected >= len(st.Ranked) {
		return
	}
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Items) {
		return
	}
	p := st.Items[entIdx]

	if d.MassRenamePhase == dialog.MassRenamePhaseOverwritePicker {
		h.overwriteMassRenamePattern(p)
		return
	}

	if p.Name != "" {
		h.massRenamePatternName = p.Name
	}
	mode := massRenameModeFromString(p.Mode)
	if d.MassRenameMode != mode {
		h.massRenameSwitchMode(d, mode)
	}
	if len(d.Fields) > 0 {
		d.Fields[0].Value = p.Find
		d.Fields[0].Cursor = len([]rune(p.Find))
		d.Fields[0].Prefill = ""
		d.Fields[0].PrefillPending = false
	}
	if len(d.Fields) > 1 {
		d.Fields[1].Value = p.Replace
		d.Fields[1].Cursor = len([]rune(p.Replace))
		d.Fields[1].Prefill = ""
		d.Fields[1].PrefillPending = false
	}
	d.MassRenameCaseFold = p.CaseFold
	d.MassRenameStripSpaces = p.StripSpaces
	d.MassRenameCapEachWord = p.CapEachWord
	d.MassRenameCapPunctSep = p.CapPunctSep
	h.MassRenameSyncFieldLabels()
	d.MassRenamePhase = dialog.MassRenamePhaseMain
	d.FocusedField = dialog.MassRenameFindFieldFocus
	h.RecomputeMassRenamePreview()
}

// deleteSelectedMassRenamePattern removes the selected entry (F8): on the load and overwrite
// pickers, a saved pattern is removed from disk; on the history picker, a history entry is spliced out of the
// in-memory list only. Returns true when the shortcut was handled (including errors shown to the
// user).
func (h *Handler) deleteSelectedMassRenamePattern() bool {
	if !h.MassRenameDeletePatternFooterEligible() {
		return false
	}
	d := &h.model.FileDialog
	st := h.currentMassRenamePickerState()
	entIdx := st.Ranked[st.Selected]
	if entIdx < 0 || entIdx >= len(st.Items) {
		return false
	}
	p := st.Items[entIdx]
	if d.MassRenamePhase == dialog.MassRenamePhaseHistoryPicker {
		h.removeMassRenameHistoryEntry(p)
		st.Items = append(st.Items[:entIdx], st.Items[entIdx+1:]...)
		h.syncMassRenamePickerRanks()
		h.host.SetTransientMessage("Removed from history", ui.MessageUrgencyInfo)
		return true
	}
	name := p.Name
	if err := ops.RemoveMassRenamePattern(h.massRenamePatternsPath(), name); err != nil {
		h.host.SetErrorMessage("Delete pattern", err)
		return true
	}
	st.Items = append(st.Items[:entIdx], st.Items[entIdx+1:]...)
	h.syncMassRenamePickerRanks()
	h.host.SetTransientMessage(fmt.Sprintf("Pattern removed: %s", name), ui.MessageUrgencyInfo)
	return true
}

// handleMassRenamePickerKey routes a key event for the open load-pattern or pattern-history
// picker (identical key handling; currentMassRenamePickerState picks the right backing state).
// Mirrors HandlePathPickerKey's shape (query editing built directly with internal/scrollquery
// since the query field lives on MassRenamePatternPickerState rather than the shared PathPicker
// state).
func (h *Handler) handleMassRenamePickerKey(event *tcell.EventKey) bool {
	st := h.currentMassRenamePickerState()

	if h.keysMassRenameDialog != nil {
		if id, ok := h.keysMassRenameDialog.Lookup(event); ok && id == keymap.ActionFileMassRenameDeletePattern {
			h.deleteSelectedMassRenamePattern()
			return false
		}
	}
	if dialog.TryStandardDialogActions(event, h.activateMassRenamePickerSelection, h.closeMassRenamePicker, nil) {
		return false
	}

	if st.Focus == 0 {
		onChange := func() {
			h.syncMassRenamePickerRanks()
			st.Selected = 0
			dialog.EnsureMassRenamePatternPickerListScroll(st, h.MassRenamePatternPickerListRows())
		}
		edit := scrollquery.NewEdit(&st.Query, &st.QueryCursor, &st.QueryScroll, h.MassRenamePatternPickerQueryWidth(), onChange)
		if scrollquery.HandleKey(h.keysDialogInput, event, true, edit) {
			return false
		}
	}

	switch event.Key() {
	case tcell.KeyEsc:
		h.closeMassRenamePicker()
	case tcell.KeyEnter:
		switch st.Focus {
		case 2:
			h.closeMassRenamePicker()
		default:
			h.activateMassRenamePickerSelection()
		}
	case tcell.KeyTab, tcell.KeyBacktab, tcell.KeyLeft, tcell.KeyRight, tcell.KeyUp, tcell.KeyDown:
		if nf, ok := dialog.ListOKCancelNavFocusKey(st.Focus, event.Key()); ok {
			st.Focus = nf
			if st.Focus == 0 && event.Key() == tcell.KeyUp {
				dialog.EnsureMassRenamePatternPickerListScroll(st, h.MassRenamePatternPickerListRows())
			}
			break
		}
		if h.host.HandleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), h.MassRenamePatternPickerListRows, func() {
			dialog.EnsureMassRenamePatternPickerListScroll(st, h.MassRenamePatternPickerListRows())
		}) {
			break
		}
	case tcell.KeyHome, tcell.KeyEnd, tcell.KeyPgUp, tcell.KeyPgDn:
		if h.host.HandleFilteredListSelectionKey(event, st.Focus, &st.Selected, len(st.Ranked), h.MassRenamePatternPickerListRows, func() {
			dialog.EnsureMassRenamePatternPickerListScroll(st, h.MassRenamePatternPickerListRows())
		}) {
			break
		}
	case tcell.KeyRune:
		if event.Modifiers() != tcell.ModNone {
			break
		}
		if st.Focus == 0 {
			break
		}
		switch dialog.DialogButtonRune(event.Rune()) {
		case dialog.ButtonRuneOK:
			h.activateMassRenamePickerSelection()
		case dialog.ButtonRuneCancel:
			h.closeMassRenamePicker()
		case dialog.ButtonRuneToggle:
			switch st.Focus {
			case 1:
				h.activateMassRenamePickerSelection()
			case 2:
				h.closeMassRenamePicker()
			}
		}
	}
	return false
}
