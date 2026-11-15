package dialog

import (
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

// Saved patterns and the session history of the twin pattern dialogs (Filter, Select group). One
// implementation drives both through a savedPatternBinding; the [dialog.pattern] chords run
// before the widget engine, per phase.

// maxPatternHistory caps each dialog's session-only list of applied patterns.
const maxPatternHistory = 20

const savedPatternsMessageTitle = "Saved patterns"

// savedPatternBinding binds one twin dialog to the saved-pattern handling. The main-screen hooks
// (mainKey, mainEvent, mainOK, close) are the dialog's own handling without saved patterns.
type savedPatternBinding struct {
	noun      string // in messages: "Filter", "Pattern"
	screens   *dialog.SavedPatternScreens
	screen    func() dialog.DialogScreen
	saved     func(name, description string) filterpattern.Pattern
	load      func(filterpattern.Pattern) // fill the main screen and focus its pattern input
	history   *[]filterpattern.Pattern
	mainKey   func(*tcell.EventKey)
	mainEvent func(dlg.Event)
	mainOK    func()
	close     func()
}

// savedPatternsPath resolves the saved patterns file; "" when no location is known.
func (h *Handler) savedPatternsPath() string {
	return filterpattern.ResolveFile(h.host.Config().Filter.File, h.configDir)
}

// filterSavedBinding binds the Filter dialog.
func (h *Handler) filterSavedBinding() savedPatternBinding {
	fd := &h.model.FilterDialog
	return savedPatternBinding{
		noun: "Filter", screens: &fd.SavedPatternScreens, screen: h.FilterDialogScreen,
		saved: fd.SavedPattern,
		load: func(p filterpattern.Pattern) {
			fd.LoadPattern(p)
			fd.Focus = dialog.PatternInput
			h.updateFilterDialogPreview()
		},
		history:   &h.filterHistory,
		mainKey:   func(ev *tcell.EventKey) { HandlePatternDialogKey(h.FilterPatternKeys(), ev) },
		mainEvent: func(ev dlg.Event) { ApplyPatternDialogEvent(h.FilterPatternKeys(), ev) },
		mainOK:    h.confirmFilterDialogFromInput, close: h.CloseFilterDialog,
	}
}

// GroupSelectHooks is the Select group dialog's own main-screen handling, which lives in the app.
type GroupSelectHooks struct {
	Key   func(*tcell.EventKey)
	Event func(dlg.Event)
	OK    func()
	Close func()
}

// groupSelectSavedBinding binds the Select group dialog. Its live preview follows the state on
// the app's post-event reconcile, so loading needs no preview call.
func (h *Handler) groupSelectSavedBinding(hooks GroupSelectHooks) savedPatternBinding {
	gs := &h.model.GroupSelect
	return savedPatternBinding{
		noun: "Pattern", screens: &gs.SavedPatternScreens, screen: h.GroupSelectScreen,
		saved: gs.SavedPattern,
		load: func(p filterpattern.Pattern) {
			gs.LoadPattern(p)
			gs.Focus = dialog.PatternInput
		},
		history: &h.groupSelectHistory,
		mainKey: hooks.Key, mainEvent: hooks.Event, mainOK: hooks.OK, close: hooks.Close,
	}
}

// openSavedBinding is the binding of whichever twin dialog is open (Filter first, as input mode
// orders them), with no main-screen hooks: for read-only queries.
func (h *Handler) openSavedBinding() (savedPatternBinding, bool) {
	switch {
	case h.model.FilterDialog.Open:
		return h.filterSavedBinding(), true
	case h.model.GroupSelect.Open:
		return h.groupSelectSavedBinding(GroupSelectHooks{}), true
	}
	return savedPatternBinding{}, false
}

// FilterDialogScreen is the Filter dialog's screen on top for the current terminal size.
func (h *Handler) FilterDialogScreen() dialog.DialogScreen {
	termW, termH := h.screen.Size()
	return h.model.FilterDialog.Screen(dialog.Layout{Width: termW, Height: termH}, h.host.Styles())
}

// GroupSelectScreen is the Select group dialog's screen on top for the current terminal size.
func (h *Handler) GroupSelectScreen() dialog.DialogScreen {
	termW, termH := h.screen.Size()
	return h.model.GroupSelect.Screen(dialog.Layout{Width: termW, Height: termH}, h.host.Styles())
}

// PatternDialogPhase is the open twin dialog's screen on top (footer hints).
func (h *Handler) PatternDialogPhase() dialog.PatternPhase {
	if b, ok := h.openSavedBinding(); ok {
		return b.screens.Phase
	}
	return dialog.PatternPhaseMain
}

// PatternDeletePatternEligible reports whether the open twin dialog's picker has an entry under
// its cursor.
func (h *Handler) PatternDeletePatternEligible() bool {
	b, ok := h.openSavedBinding()
	if !ok || !dialog.PatternPickerPhase(b.screens.Phase) {
		return false
	}
	_, ok = selectedSavedPattern(b.screens)
	return ok
}

// HandleFilterDialogKey routes a key event for the open Filter dialog's screen. Returns whether
// the app should quit (always false here).
func (h *Handler) HandleFilterDialogKey(event *tcell.EventKey) bool {
	h.handleSavedKey(h.filterSavedBinding(), event)
	return false
}

// ApplyFilterDialogEvent acts on a widget event of the open Filter dialog's screen (a click).
func (h *Handler) ApplyFilterDialogEvent(ev dlg.Event) { h.applySavedEvent(h.filterSavedBinding(), ev) }

// FilterDialogOK presses OK on the Filter dialog's screen on top.
func (h *Handler) FilterDialogOK() { h.savedOK(h.filterSavedBinding()) }

// FilterDialogCancel presses Cancel on the Filter dialog's screen on top.
func (h *Handler) FilterDialogCancel() { h.savedCancel(h.filterSavedBinding()) }

// HandleGroupSelectKey routes a key event for the open Select group dialog's screen.
func (h *Handler) HandleGroupSelectKey(hooks GroupSelectHooks, event *tcell.EventKey) {
	h.handleSavedKey(h.groupSelectSavedBinding(hooks), event)
}

// ApplyGroupSelectEvent acts on a widget event of the open Select group dialog's screen.
func (h *Handler) ApplyGroupSelectEvent(hooks GroupSelectHooks, ev dlg.Event) {
	h.applySavedEvent(h.groupSelectSavedBinding(hooks), ev)
}

// GroupSelectOK presses OK on the Select group dialog's screen on top.
func (h *Handler) GroupSelectOK(hooks GroupSelectHooks) { h.savedOK(h.groupSelectSavedBinding(hooks)) }

// GroupSelectCancel presses Cancel on the Select group dialog's screen on top.
func (h *Handler) GroupSelectCancel(hooks GroupSelectHooks) {
	h.savedCancel(h.groupSelectSavedBinding(hooks))
}

// RecordGroupSelectHistory records the Select group dialog's pattern in its session history; the
// app calls it on a valid, non-empty apply.
func (h *Handler) RecordGroupSelectHistory() {
	recordPatternHistory(h.groupSelectSavedBinding(GroupSelectHooks{}))
}

func (h *Handler) handleSavedKey(b savedPatternBinding, event *tcell.EventKey) {
	action, _ := h.keysPatternDialog.Lookup(event)
	switch phase := b.screens.Phase; {
	case dialog.PatternPickerPhase(phase):
		if action == keymap.ActionPatternDelete {
			h.deleteSelectedSavedPattern(b)
			return
		}
		w, ht := h.screen.Size()
		scrollquery.HandlePickerKey(w, ht, b.screen().Spec, h.keysDialogInput, event, h.savedPickerBinding(b))
	case phase == dialog.PatternPhaseSavePrompt:
		if action == keymap.ActionPatternSave {
			h.openSavedPicker(b, dialog.PatternPhaseOverwritePicker)
			return
		}
		h.handleSavePromptKey(b, event)
	default:
		switch action {
		case keymap.ActionPatternLoad:
			h.openSavedPicker(b, dialog.PatternPhaseLoadPicker)
			return
		case keymap.ActionPatternHistory:
			h.openSavedPicker(b, dialog.PatternPhaseHistoryPicker)
			return
		case keymap.ActionPatternSave:
			b.screens.SaveName, b.screens.SaveDesc = dialog.PromptField{}, dialog.PromptField{}
			setPatternPhase(b.screens, dialog.PatternPhaseSavePrompt)
			return
		}
		b.mainKey(event)
	}
}

func (h *Handler) applySavedEvent(b savedPatternBinding, ev dlg.Event) {
	switch phase := b.screens.Phase; {
	case dialog.PatternPickerPhase(phase):
		scrollquery.ApplyPickerEvent(ev, h.savedPickerBinding(b))
	case phase == dialog.PatternPhaseSavePrompt:
		h.applySavePromptEvent(b, ev)
	default:
		b.mainEvent(ev)
	}
}

func (h *Handler) savedOK(b savedPatternBinding) {
	switch phase := b.screens.Phase; {
	case dialog.PatternPickerPhase(phase):
		h.activateSavedPicker(b)
	case phase == dialog.PatternPhaseSavePrompt:
		h.confirmSavePrompt(b)
	default:
		b.mainOK()
	}
}

// savedCancel: a sub-screen returns to where it was opened from, the main screen closes the dialog.
func (h *Handler) savedCancel(b savedPatternBinding) {
	switch b.screens.Phase {
	case dialog.PatternPhaseMain:
		b.close()
	case dialog.PatternPhaseOverwritePicker:
		setPatternPhase(b.screens, dialog.PatternPhaseSavePrompt)
	default:
		setPatternPhase(b.screens, dialog.PatternPhaseMain)
	}
}

// setPatternPhase puts phase on top, with the focus on its first widget. The main screen keeps
// the focus it had.
func setPatternPhase(s *dialog.SavedPatternScreens, phase dialog.PatternPhase) {
	s.Phase = phase
	s.ScreenFocus = ""
}

// confirmSavePrompt saves the dialog's pattern under the typed name, replacing a saved pattern of
// that name.
func (h *Handler) confirmSavePrompt(b savedPatternBinding) {
	name := strings.TrimSpace(b.screens.SaveName.Value)
	if name == "" {
		h.host.SetTransientMessage("Name is required", ui.MessageUrgencyWarn)
		b.screens.ScreenFocus = dialog.PatternSaveNameInput
		return
	}
	h.savePattern(b, name, strings.TrimSpace(b.screens.SaveDesc.Value), " saved: ")
}

// savePattern writes the dialog's pattern as name and returns to the main screen.
func (h *Handler) savePattern(b savedPatternBinding, name, description, done string) {
	if err := filterpattern.Upsert(h.savedPatternsPath(), b.saved(name, description)); err != nil {
		h.host.SetErrorMessage(savedPatternsMessageTitle, err)
		return
	}
	h.savedPatternName = name
	setPatternPhase(b.screens, dialog.PatternPhaseMain)
	h.host.SetTransientMessage(b.noun+done+name, ui.MessageUrgencySuccess)
}

// focusedPromptField is the save prompt's focused input (Name when the focus is on neither).
func focusedPromptField(s *dialog.SavedPatternScreens) (dlg.ID, *dialog.PromptField) {
	if s.ScreenFocus == dialog.PatternSaveDescInput {
		return dialog.PatternSaveDescInput, &s.SaveDesc
	}
	return dialog.PatternSaveNameInput, &s.SaveName
}

func (h *Handler) handleSavePromptKey(b savedPatternBinding, event *tcell.EventKey) {
	spec := b.screen().Spec
	if ev, ok := dlg.HandleKey(spec, &b.screens.ScreenFocus, event); ok {
		h.applySavePromptEvent(b, ev)
		return
	}
	if b.screens.ScreenFocus == dlg.OK || b.screens.ScreenFocus == dlg.Cancel {
		return
	}
	id, field := focusedPromptField(b.screens)
	w, ht := h.screen.Size()
	edit := scrollquery.NewEdit(&field.Value, &field.Cursor, &field.Scroll, max(dlg.InputTextWidth(w, ht, spec, id), 10), func() {})
	scrollquery.HandleKey(h.keysDialogInput, event, true, edit)
}

func (h *Handler) applySavePromptEvent(b savedPatternBinding, ev dlg.Event) {
	switch {
	case ev.Kind == dlg.EventCaret:
		if ev.ID == dialog.PatternSaveDescInput {
			b.screens.SaveDesc.Cursor = ev.Index
		} else {
			b.screens.SaveName.Cursor = ev.Index
		}
	case ev.ID == dlg.Cancel:
		h.savedCancel(b)
	case ev.ID == dlg.OK, ev.ID == dialog.PatternSaveNameInput, ev.ID == dialog.PatternSaveDescInput:
		h.confirmSavePrompt(b)
	}
}

// openSavedPicker shows a picker phase: the saved patterns (read from disk each time, like the
// mass rename patterns) or the dialog's session history. The saved-pattern pickers open on the
// pattern last loaded or saved.
func (h *Handler) openSavedPicker(b savedPatternBinding, phase dialog.PatternPhase) {
	var items []filterpattern.Pattern
	if phase == dialog.PatternPhaseHistoryPicker {
		items = slices.Clone(*b.history) // deleting goes through the history, not this copy
	} else {
		var err error
		if items, err = filterpattern.Load(h.savedPatternsPath()); err != nil {
			h.host.SetErrorMessage(savedPatternsMessageTitle, err)
			return
		}
		if phase == dialog.PatternPhaseOverwritePicker && len(items) == 0 {
			h.host.SetTransientMessage("No saved patterns", ui.MessageUrgencyWarn)
			return
		}
	}
	b.screens.Picker = dialog.SavedPatternPicker{Items: items}
	setPatternPhase(b.screens, phase)
	h.syncSavedPickerRanks(b)
	if phase == dialog.PatternPhaseHistoryPicker || h.savedPatternName == "" {
		return
	}
	pk := &b.screens.Picker
	for i, idx := range pk.Ranked {
		if idx >= 0 && idx < len(pk.Items) && pk.Items[idx].Name == h.savedPatternName {
			pk.Selected = i
			h.ensureSavedPickerScroll(b)
			return
		}
	}
}

func (h *Handler) savedPickerBinding(b savedPatternBinding) scrollquery.PickerBinding {
	pk := &b.screens.Picker
	return scrollquery.PickerBinding{
		Focus: &b.screens.ScreenFocus, Query: &pk.Query, Cursor: &pk.QueryCursor, Scroll: &pk.QueryScroll,
		Selected: &pk.Selected, Offset: &pk.ListScroll, Count: func() int { return len(pk.Ranked) },
		Resync:      func() { h.syncSavedPickerRanks(b) },
		Activate:    func() { h.activateSavedPicker(b) },
		ClosePicker: func() { h.savedCancel(b) },
	}
}

// syncSavedPickerRanks re-ranks the picker's items against its query and keeps the cursor visible.
func (h *Handler) syncSavedPickerRanks(b savedPatternBinding) {
	pk := &b.screens.Picker
	lines := make([]string, len(pk.Items))
	for i, p := range pk.Items {
		lines[i] = dialog.SavedPatternLine(p)
	}
	pk.Ranked, pk.MatchRanges = h.host.SyncFilteredListRanks(lines, pk.Query, len(pk.Items), h.host.Config().Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&pk.Selected, len(pk.Ranked))
	h.ensureSavedPickerScroll(b)
}

func (h *Handler) ensureSavedPickerScroll(b savedPatternBinding) {
	pk := &b.screens.Picker
	w, ht := h.screen.Size()
	rows, _ := scrollquery.PickerGeometry(w, ht, b.screen().Spec)
	dialog.EnsurePickerScroll(len(pk.Ranked), &pk.Selected, &pk.ListScroll, rows)
}

// selectedSavedPattern is the index into Items of the pattern under the picker's cursor.
func selectedSavedPattern(s *dialog.SavedPatternScreens) (int, bool) {
	pk := &s.Picker
	if pk.Selected < 0 || pk.Selected >= len(pk.Ranked) {
		return 0, false
	}
	i := pk.Ranked[pk.Selected]
	return i, i >= 0 && i < len(pk.Items)
}

// activateSavedPicker fills the dialog from the picked pattern (load, history), or overwrites the
// picked saved pattern with the dialog's (overwrite), keeping its description unless one was
// typed in the save prompt.
func (h *Handler) activateSavedPicker(b savedPatternBinding) {
	i, ok := selectedSavedPattern(b.screens)
	if !ok {
		return
	}
	p := b.screens.Picker.Items[i]
	if b.screens.Phase == dialog.PatternPhaseOverwritePicker {
		description := p.Description
		if typed := strings.TrimSpace(b.screens.SaveDesc.Value); typed != "" {
			description = typed
		}
		h.savePattern(b, p.Name, description, " overwritten: ")
		return
	}
	if p.Name != "" {
		h.savedPatternName = p.Name
	}
	setPatternPhase(b.screens, dialog.PatternPhaseMain)
	b.load(p)
}

// deleteSelectedSavedPattern removes the pattern under the picker's cursor: from the session
// history in the history picker, from the saved patterns file otherwise.
func (h *Handler) deleteSelectedSavedPattern(b savedPatternBinding) {
	i, ok := selectedSavedPattern(b.screens)
	if !ok {
		return
	}
	pk := &b.screens.Picker
	p := pk.Items[i]
	if b.screens.Phase == dialog.PatternPhaseHistoryPicker {
		*b.history = slices.DeleteFunc(slices.Clone(*b.history), p.Equal)
		pk.Items = slices.Delete(pk.Items, i, i+1)
		h.syncSavedPickerRanks(b)
		h.host.SetTransientMessage("Removed from history", ui.MessageUrgencyInfo)
		return
	}
	if err := filterpattern.Remove(h.savedPatternsPath(), p.Name); err != nil {
		h.host.SetErrorMessage(savedPatternsMessageTitle, err)
		return
	}
	pk.Items = slices.Delete(pk.Items, i, i+1)
	h.syncSavedPickerRanks(b)
	h.host.SetTransientMessage(b.noun+" deleted: "+p.Name, ui.MessageUrgencyInfo)
}

// recordPatternHistory puts the dialog's pattern first in its session history, dropping an equal
// older entry and capping the list at maxPatternHistory.
func recordPatternHistory(b savedPatternBinding) {
	p := b.saved("", "")
	hist := make([]filterpattern.Pattern, 0, len(*b.history)+1)
	hist = append(hist, p)
	for _, e := range *b.history {
		if !e.Equal(p) {
			hist = append(hist, e)
		}
	}
	if len(hist) > maxPatternHistory {
		hist = hist[:maxPatternHistory]
	}
	*b.history = hist
}
