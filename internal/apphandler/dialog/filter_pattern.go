package dialog

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/filterpattern"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// maxFilterHistory caps the session-only list of applied Filter dialog patterns.
const maxFilterHistory = 20

const filterPatternsMessageTitle = "Saved filters"

// filterPatternsPath resolves the saved filters file; "" when no location is known.
func (h *Handler) filterPatternsPath() string {
	return filterpattern.ResolveFile(h.host.Config().Filter.File, h.configDir)
}

// FilterDialogScreen is the Filter dialog's screen on top (main, picker or save prompt) for the
// current terminal size.
func (h *Handler) FilterDialogScreen() dialog.DialogScreen {
	termW, termH := h.screen.Size()
	return h.model.FilterDialog.Screen(dialog.Layout{Width: termW, Height: termH}, h.host.Styles())
}

// FilterDialogPhase is the Filter dialog's screen on top (footer hints).
func (h *Handler) FilterDialogPhase() dialog.FilterPhase { return h.model.FilterDialog.Phase }

// FilterDeletePatternEligible reports whether a Filter picker has an entry under its cursor.
func (h *Handler) FilterDeletePatternEligible() bool {
	fd := &h.model.FilterDialog
	_, ok := h.selectedFilterPattern()
	return fd.Open && dialog.FilterPickerPhase(fd.Phase) && ok
}

// HandleFilterDialogKey routes a key event for the open Filter dialog's screen. Returns whether
// the app should quit (always false here). The [dialog.filter] chords run before the widget
// engine; on the main screen the shared pattern dialog handling does the rest.
func (h *Handler) HandleFilterDialogKey(event *tcell.EventKey) bool {
	fd := &h.model.FilterDialog
	action, _ := h.keysFilterDialog.Lookup(event)
	switch {
	case dialog.FilterPickerPhase(fd.Phase):
		if action == keymap.ActionFilterDeletePattern {
			h.deleteSelectedFilterPattern()
			return false
		}
		w, ht := h.screen.Size()
		scrollquery.HandlePickerKey(w, ht, h.FilterDialogScreen().Spec, h.keysDialogInput, event, h.filterPickerBinding())
	case fd.Phase == dialog.FilterPhaseSavePrompt:
		if action == keymap.ActionFilterSavePattern {
			h.openFilterOverwritePicker()
			return false
		}
		h.handleFilterSavePromptKey(event)
	default:
		switch action {
		case keymap.ActionFilterLoadPattern:
			h.openFilterLoadPicker()
			return false
		case keymap.ActionFilterHistory:
			h.openFilterHistoryPicker()
			return false
		case keymap.ActionFilterSavePattern:
			h.openFilterSavePrompt()
			return false
		}
		HandlePatternDialogKey(h.FilterPatternKeys(), event)
	}
	return false
}

// ApplyFilterDialogEvent acts on a widget event of the open Filter dialog's screen (a mouse click).
func (h *Handler) ApplyFilterDialogEvent(ev dlg.Event) {
	fd := &h.model.FilterDialog
	switch {
	case dialog.FilterPickerPhase(fd.Phase):
		scrollquery.ApplyPickerEvent(ev, h.filterPickerBinding())
	case fd.Phase == dialog.FilterPhaseSavePrompt:
		h.applyFilterSavePromptEvent(ev)
	default:
		ApplyPatternDialogEvent(h.FilterPatternKeys(), ev)
	}
}

// FilterDialogOK presses OK on the Filter dialog's screen on top.
func (h *Handler) FilterDialogOK() {
	fd := &h.model.FilterDialog
	switch {
	case dialog.FilterPickerPhase(fd.Phase):
		h.activateFilterPickerSelection()
	case fd.Phase == dialog.FilterPhaseSavePrompt:
		h.confirmFilterSavePrompt()
	default:
		h.confirmFilterDialogFromInput()
	}
}

// FilterDialogCancel presses Cancel on the Filter dialog's screen on top: a sub-screen returns to
// where it was opened from, the main screen closes the dialog.
func (h *Handler) FilterDialogCancel() {
	switch h.model.FilterDialog.Phase {
	case dialog.FilterPhaseMain:
		h.CloseFilterDialog()
	case dialog.FilterPhaseOverwritePicker:
		h.setFilterPhase(dialog.FilterPhaseSavePrompt)
	default:
		h.setFilterPhase(dialog.FilterPhaseMain)
	}
}

// setFilterPhase puts phase on top, with the focus on its first widget. The main screen keeps
// the focus it had.
func (h *Handler) setFilterPhase(phase dialog.FilterPhase) {
	fd := &h.model.FilterDialog
	fd.Phase = phase
	fd.ScreenFocus = ""
}

// openFilterSavePrompt asks for the name and description to save the dialog's pattern under.
func (h *Handler) openFilterSavePrompt() {
	fd := &h.model.FilterDialog
	fd.SaveName, fd.SaveDesc = dialog.PromptField{}, dialog.PromptField{}
	h.setFilterPhase(dialog.FilterPhaseSavePrompt)
}

// confirmFilterSavePrompt saves the dialog's pattern under the typed name, replacing a saved
// pattern of that name.
func (h *Handler) confirmFilterSavePrompt() {
	fd := &h.model.FilterDialog
	name := strings.TrimSpace(fd.SaveName.Value)
	if name == "" {
		h.host.SetTransientMessage("Name is required", ui.MessageUrgencyWarn)
		fd.ScreenFocus = dialog.FilterSaveNameInput
		return
	}
	h.saveFilterPattern(name, strings.TrimSpace(fd.SaveDesc.Value), "Filter saved: ")
}

// saveFilterPattern writes the dialog's pattern as name and returns to the main screen.
func (h *Handler) saveFilterPattern(name, description, done string) {
	fd := &h.model.FilterDialog
	if err := filterpattern.Upsert(h.filterPatternsPath(), fd.SavedPattern(name, description)); err != nil {
		h.host.SetErrorMessage(filterPatternsMessageTitle, err)
		return
	}
	h.filterPatternName = name
	h.setFilterPhase(dialog.FilterPhaseMain)
	h.host.SetTransientMessage(done+name, ui.MessageUrgencySuccess)
}

// focusedFilterPromptField is the save prompt's focused input (Name when the focus is on neither).
func (h *Handler) focusedFilterPromptField() (dlg.ID, *dialog.PromptField) {
	fd := &h.model.FilterDialog
	if fd.ScreenFocus == dialog.FilterSaveDescInput {
		return dialog.FilterSaveDescInput, &fd.SaveDesc
	}
	return dialog.FilterSaveNameInput, &fd.SaveName
}

func (h *Handler) handleFilterSavePromptKey(event *tcell.EventKey) {
	fd := &h.model.FilterDialog
	spec := h.FilterDialogScreen().Spec
	if ev, ok := dlg.HandleKey(spec, &fd.ScreenFocus, event); ok {
		h.applyFilterSavePromptEvent(ev)
		return
	}
	if fd.ScreenFocus == dlg.OK || fd.ScreenFocus == dlg.Cancel {
		return
	}
	id, field := h.focusedFilterPromptField()
	w, ht := h.screen.Size()
	edit := scrollquery.NewEdit(&field.Value, &field.Cursor, &field.Scroll, max(dlg.InputTextWidth(w, ht, spec, id), 10), func() {})
	scrollquery.HandleKey(h.keysDialogInput, event, true, edit)
}

func (h *Handler) applyFilterSavePromptEvent(ev dlg.Event) {
	fd := &h.model.FilterDialog
	switch {
	case ev.Kind == dlg.EventCaret:
		if ev.ID == dialog.FilterSaveDescInput {
			fd.SaveDesc.Cursor = ev.Index
		} else {
			fd.SaveName.Cursor = ev.Index
		}
	case ev.ID == dlg.Cancel:
		h.FilterDialogCancel()
	case ev.ID == dlg.OK, ev.ID == dialog.FilterSaveNameInput, ev.ID == dialog.FilterSaveDescInput:
		h.confirmFilterSavePrompt()
	}
}

// openFilterPicker shows items in the picker phase, the cursor on the filter last loaded or saved.
func (h *Handler) openFilterPicker(phase dialog.FilterPhase, items []filterpattern.Pattern) {
	fd := &h.model.FilterDialog
	fd.Picker = dialog.FilterPatternPicker{Items: items}
	h.setFilterPhase(phase)
	h.syncFilterPickerRanks()
	if phase == dialog.FilterPhaseHistoryPicker || h.filterPatternName == "" {
		return
	}
	pk := &fd.Picker
	for i, idx := range pk.Ranked {
		if idx >= 0 && idx < len(pk.Items) && pk.Items[idx].Name == h.filterPatternName {
			pk.Selected = i
			h.ensureFilterPickerScroll()
			return
		}
	}
}

// openFilterLoadPicker lists the saved filters, read from disk each time (like the mass rename
// patterns).
func (h *Handler) openFilterLoadPicker() {
	items, err := filterpattern.Load(h.filterPatternsPath())
	if err != nil {
		h.host.SetErrorMessage(filterPatternsMessageTitle, err)
		return
	}
	h.openFilterPicker(dialog.FilterPhaseLoadPicker, items)
}

// openFilterOverwritePicker lists the saved filters from the save prompt; picking one writes the
// dialog's pattern over it.
func (h *Handler) openFilterOverwritePicker() {
	items, err := filterpattern.Load(h.filterPatternsPath())
	if err != nil {
		h.host.SetErrorMessage(filterPatternsMessageTitle, err)
		return
	}
	if len(items) == 0 {
		h.host.SetTransientMessage("No saved filters", ui.MessageUrgencyWarn)
		return
	}
	h.openFilterPicker(dialog.FilterPhaseOverwritePicker, items)
}

// openFilterHistoryPicker lists the filters applied this session. Items is a copy, so deleting
// from the picker goes through removeFilterHistoryEntry.
func (h *Handler) openFilterHistoryPicker() {
	h.openFilterPicker(dialog.FilterPhaseHistoryPicker, slices.Clone(h.filterHistory))
}

func (h *Handler) filterPickerBinding() scrollquery.PickerBinding {
	fd := &h.model.FilterDialog
	pk := &fd.Picker
	return scrollquery.PickerBinding{
		Focus: &fd.ScreenFocus, Query: &pk.Query, Cursor: &pk.QueryCursor, Scroll: &pk.QueryScroll,
		Selected: &pk.Selected, Offset: &pk.ListScroll, Count: func() int { return len(pk.Ranked) },
		Resync: h.syncFilterPickerRanks, Activate: h.activateFilterPickerSelection, ClosePicker: h.FilterDialogCancel,
	}
}

// syncFilterPickerRanks re-ranks the picker's items against its query and keeps the cursor visible.
func (h *Handler) syncFilterPickerRanks() {
	pk := &h.model.FilterDialog.Picker
	lines := make([]string, len(pk.Items))
	for i, p := range pk.Items {
		lines[i] = dialog.FilterPatternLine(p)
	}
	pk.Ranked, pk.MatchRanges = h.host.SyncFilteredListRanks(lines, pk.Query, len(pk.Items), h.host.Config().Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&pk.Selected, len(pk.Ranked))
	h.ensureFilterPickerScroll()
}

func (h *Handler) ensureFilterPickerScroll() {
	pk := &h.model.FilterDialog.Picker
	w, ht := h.screen.Size()
	rows, _ := scrollquery.PickerGeometry(w, ht, h.FilterDialogScreen().Spec)
	dialog.EnsurePickerScroll(len(pk.Ranked), &pk.Selected, &pk.ListScroll, rows)
}

// selectedFilterPattern is the pattern under the picker's cursor and its index into Items.
func (h *Handler) selectedFilterPattern() (int, bool) {
	pk := &h.model.FilterDialog.Picker
	if pk.Selected < 0 || pk.Selected >= len(pk.Ranked) {
		return 0, false
	}
	i := pk.Ranked[pk.Selected]
	return i, i >= 0 && i < len(pk.Items)
}

// activateFilterPickerSelection fills the dialog from the picked pattern (load, history), or
// overwrites the picked saved pattern with the dialog's (overwrite), keeping its description
// unless one was typed in the save prompt.
func (h *Handler) activateFilterPickerSelection() {
	fd := &h.model.FilterDialog
	i, ok := h.selectedFilterPattern()
	if !ok {
		return
	}
	p := fd.Picker.Items[i]
	if fd.Phase == dialog.FilterPhaseOverwritePicker {
		description := p.Description
		if typed := strings.TrimSpace(fd.SaveDesc.Value); typed != "" {
			description = typed
		}
		h.saveFilterPattern(p.Name, description, "Filter overwritten: ")
		return
	}
	if p.Name != "" {
		h.filterPatternName = p.Name
	}
	fd.LoadPattern(p)
	fd.Focus = dialog.PatternInput
	h.setFilterPhase(dialog.FilterPhaseMain)
	h.updateFilterDialogPreview()
}

// deleteSelectedFilterPattern removes the pattern under the picker's cursor: from the session
// history in the history picker, from the saved filters file otherwise.
func (h *Handler) deleteSelectedFilterPattern() {
	fd := &h.model.FilterDialog
	i, ok := h.selectedFilterPattern()
	if !ok {
		return
	}
	pk := &fd.Picker
	p := pk.Items[i]
	if fd.Phase == dialog.FilterPhaseHistoryPicker {
		h.removeFilterHistoryEntry(p)
		pk.Items = slices.Delete(pk.Items, i, i+1)
		h.syncFilterPickerRanks()
		h.host.SetTransientMessage("Removed from history", ui.MessageUrgencyInfo)
		return
	}
	if err := filterpattern.Remove(h.filterPatternsPath(), p.Name); err != nil {
		h.host.SetErrorMessage(filterPatternsMessageTitle, err)
		return
	}
	pk.Items = slices.Delete(pk.Items, i, i+1)
	h.syncFilterPickerRanks()
	h.host.SetTransientMessage(fmt.Sprintf("Filter deleted: %s", p.Name), ui.MessageUrgencyInfo)
}

// recordFilterHistory puts the dialog's pattern first in the session history, dropping an equal
// older entry and capping the list at maxFilterHistory.
func (h *Handler) recordFilterHistory() {
	p := h.model.FilterDialog.SavedPattern("", "")
	hist := make([]filterpattern.Pattern, 0, len(h.filterHistory)+1)
	hist = append(hist, p)
	for _, e := range h.filterHistory {
		if !e.Equal(p) {
			hist = append(hist, e)
		}
	}
	if len(hist) > maxFilterHistory {
		hist = hist[:maxFilterHistory]
	}
	h.filterHistory = hist
}

// removeFilterHistoryEntry drops entries equal to p from the session history.
func (h *Handler) removeFilterHistoryEntry(p filterpattern.Pattern) {
	h.filterHistory = slices.DeleteFunc(slices.Clone(h.filterHistory), p.Equal)
}
