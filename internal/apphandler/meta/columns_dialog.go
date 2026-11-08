package meta

import (
	"fmt"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/columnprofile"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/scrollquery"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const columnsMessageTitle = "Column profiles"

// OpenDialog opens the Columns dialog for the given panel.
func (h *Handler) OpenDialog(panelID int) {
	if h.model.ViewMode != ui.ViewBrowser {
		return
	}
	// Clear before loading so the load's own warnings (untrusted or invalid meta.toml) stay up.
	h.host.ClearTransientMessage()
	h.reloadProfiles()
	res := h.loadMetaFile(panelID)
	entries := entriesFromFile(res.MF)
	active := h.activeEntries[panelID]
	h.model.ColumnsDialog = dialog.ColumnsDialogState{
		Open:           true,
		PanelID:        panelID,
		FileColumns:    h.host.PanelByID(panelID).Columns,
		Entries:        entries,
		Checked:        checkedFor(entries, active),
		HiddenMeta:     hiddenMetaNames(entries, active),
		MarkerWidth:    h.host.DialogMarkerWidth(),
		SearchIcon:     h.host.PickerSearchIcon(),
		DefaultProfile: h.config.Panels.DefaultColumnProfile,
	}
}

// checkedFor returns the checkbox state of entries with names ticked.
func checkedFor(entries []dialog.MetaEntry, names []string) []bool {
	checked := make([]bool, len(entries))
	for i, e := range entries {
		checked[i] = slices.Contains(names, e.Name)
	}
	return checked
}

// hiddenMetaNames returns the names no entry offers, in the given order.
func hiddenMetaNames(entries []dialog.MetaEntry, names []string) []string {
	var hidden []string
	for _, name := range names {
		if entryIndexByName(entries, name) < 0 {
			hidden = append(hidden, name)
		}
	}
	return hidden
}

// checkedNames returns the names of the checked entries, in entry order.
func checkedNames(entries []dialog.MetaEntry, checked []bool) []string {
	var names []string
	for i, e := range entries {
		if i < len(checked) && checked[i] {
			names = append(names, e.Name)
		}
	}
	return names
}

// dialogMetaNames is the dialog's meta selection: the ticked entries, then the kept hidden names.
func dialogMetaNames(st *dialog.ColumnsDialogState) []string {
	return append(checkedNames(st.Entries, st.Checked), st.HiddenMeta...)
}

// ActivateSelection applies the checkboxes to the panel and closes the dialog (OK).
func (h *Handler) ActivateSelection() {
	st := h.model.ColumnsDialog
	names := dialogMetaNames(&st)
	h.CloseDialog()
	h.applyColumns(st.PanelID, st.FileColumns, h.matchingProfile(st.FileColumns, names), names)
}

// DialogOK presses OK on the dialog's current phase.
func (h *Handler) DialogOK() {
	switch h.model.ColumnsDialog.Phase {
	case dialog.ColumnsPhaseLoadPicker, dialog.ColumnsPhaseOverwritePicker:
		h.activatePickerSelection()
	case dialog.ColumnsPhaseSavePrompt:
		h.confirmSavePrompt()
	default:
		h.ActivateSelection()
	}
}

// DialogCancel presses Cancel on the dialog's current phase: a sub-screen returns to where it came
// from, the main screen closes without applying.
func (h *Handler) DialogCancel() {
	st := &h.model.ColumnsDialog
	switch st.Phase {
	case dialog.ColumnsPhaseOverwritePicker:
		h.setPhase(dialog.ColumnsPhaseSavePrompt)
	case dialog.ColumnsPhaseLoadPicker, dialog.ColumnsPhaseSavePrompt:
		h.setPhase(dialog.ColumnsPhaseMain)
	default:
		h.CloseDialog()
	}
}

func (h *Handler) setPhase(phase dialog.ColumnsPhase) {
	st := &h.model.ColumnsDialog
	st.Phase = phase
	st.Focus = ""
}

func (h *Handler) dialogSpec() dlg.Spec {
	_, screenH := h.screen.Size()
	return dialog.ColumnsDialogView(h.model.ColumnsDialog, screenH)
}

// HandleDialogKey routes a key event to the open Columns dialog.
func (h *Handler) HandleDialogKey(event *tcell.EventKey) {
	st := &h.model.ColumnsDialog
	action, _ := h.keysColumnsDialog.Lookup(event)
	switch st.Phase {
	case dialog.ColumnsPhaseLoadPicker, dialog.ColumnsPhaseOverwritePicker:
		if st.Phase == dialog.ColumnsPhaseLoadPicker {
			switch action {
			case keymap.ActionColumnsDeleteProfile:
				h.deleteSelectedProfile()
				return
			case keymap.ActionColumnsDefaultProfile:
				h.setSelectedProfileDefault()
				return
			}
		}
		w, screenH := h.screen.Size()
		scrollquery.HandlePickerKey(w, screenH, h.dialogSpec(), h.keysDialogInput, event, h.pickerBinding())
	case dialog.ColumnsPhaseSavePrompt:
		if action == keymap.ActionColumnsSaveProfile {
			h.openOverwritePicker()
			return
		}
		h.handleSavePromptKey(event)
	default:
		if event.Key() == tcell.KeyF9 {
			h.editConfigFromDialog()
			return
		}
		switch action {
		case keymap.ActionColumnsSaveProfile:
			h.setPhase(dialog.ColumnsPhaseSavePrompt)
			return
		case keymap.ActionColumnsLoadProfile:
			h.openLoadPicker()
			return
		}
		if ev, ok := dlg.HandleKey(h.dialogSpec(), &st.Focus, event); ok {
			h.ApplyDialogEvent(ev)
		}
	}
}

// ApplyDialogEvent acts on a widget event of the open Columns dialog (key or click).
func (h *Handler) ApplyDialogEvent(ev dlg.Event) {
	st := &h.model.ColumnsDialog
	switch st.Phase {
	case dialog.ColumnsPhaseLoadPicker, dialog.ColumnsPhaseOverwritePicker:
		scrollquery.ApplyPickerEvent(ev, h.pickerBinding())
		return
	case dialog.ColumnsPhaseSavePrompt:
		h.applySavePromptEvent(ev)
		return
	}
	switch ev.ID {
	case dlg.OK:
		h.ActivateSelection()
	case dlg.Cancel:
		h.CloseDialog()
	default:
		st.Apply(ev)
	}
}

// profilesPath resolves the column profiles file; "" when no location is known.
func (h *Handler) profilesPath() string {
	return columnprofile.ResolveFile(h.config.Columns.File, h.configDir)
}

// reloadProfiles re-reads the saved profiles; on failure it warns and keeps the ones it has.
func (h *Handler) reloadProfiles() {
	saved, err := columnprofile.Load(h.profilesPath())
	if err != nil {
		h.host.SetErrorMessage(columnsMessageTitle, err)
		return
	}
	h.profiles = saved
}

func (h *Handler) pickerBinding() scrollquery.PickerBinding {
	pk := &h.model.ColumnsDialog.Picker
	return scrollquery.PickerBinding{
		Focus: &h.model.ColumnsDialog.Focus, Query: &pk.Query, Cursor: &pk.QueryCursor, Scroll: &pk.QueryScroll,
		Selected: &pk.Selected, Offset: &pk.ListScroll, Count: func() int { return len(pk.Ranked) },
		Resync: h.syncPickerRanks, Activate: h.activatePickerSelection, ClosePicker: h.DialogCancel,
	}
}

func (h *Handler) syncPickerRanks() {
	st := &h.model.ColumnsDialog
	pk := &st.Picker
	lines := make([]string, len(pk.Items))
	for i, p := range pk.Items {
		lines[i] = dialog.ColumnsProfileLine(p, st.DefaultProfile)
	}
	pk.Ranked, pk.MatchRanges = h.host.SyncFilteredListRanks(lines, pk.Query, len(pk.Items), h.config.Filter.CaseInsensitive)
	h.host.ClampFilteredListSelection(&pk.Selected, len(pk.Ranked))
	w, screenH := h.screen.Size()
	rows, _ := scrollquery.PickerGeometry(w, screenH, h.dialogSpec())
	dialog.EnsurePickerScroll(len(pk.Ranked), &pk.Selected, &pk.ListScroll, rows)
}

func (h *Handler) openPicker(phase dialog.ColumnsPhase, items []columnprofile.Profile) {
	st := &h.model.ColumnsDialog
	st.Picker = dialog.ColumnsProfilePicker{Items: items}
	h.setPhase(phase)
	h.syncPickerRanks()
}

func (h *Handler) openLoadPicker() {
	h.reloadProfiles()
	h.openPicker(dialog.ColumnsPhaseLoadPicker, columnprofile.All(h.profiles))
}

func (h *Handler) openOverwritePicker() {
	h.reloadProfiles()
	var saved []columnprofile.Profile
	for _, p := range columnprofile.All(h.profiles) {
		if !p.Builtin {
			saved = append(saved, p)
		}
	}
	if len(saved) == 0 {
		h.host.SetTransientMessage("No saved profiles", ui.MessageUrgencyWarn)
		return
	}
	h.openPicker(dialog.ColumnsPhaseOverwritePicker, saved)
}

// selectedProfile is the profile under the picker's cursor.
func (h *Handler) selectedProfile() (columnprofile.Profile, bool) {
	pk := &h.model.ColumnsDialog.Picker
	if pk.Selected < 0 || pk.Selected >= len(pk.Ranked) {
		return columnprofile.Profile{}, false
	}
	i := pk.Ranked[pk.Selected]
	if i < 0 || i >= len(pk.Items) {
		return columnprofile.Profile{}, false
	}
	return pk.Items[i], true
}

// activatePickerSelection fills the checkboxes from the picked profile, or overwrites it with them.
func (h *Handler) activatePickerSelection() {
	st := &h.model.ColumnsDialog
	profile, ok := h.selectedProfile()
	if !ok {
		return
	}
	if st.Phase == dialog.ColumnsPhaseOverwritePicker {
		h.saveProfile(profile.Name, profile.Description)
		return
	}
	set, unknown := profile.Set()
	if len(unknown) > 0 {
		h.host.SetTransientMessage(fmt.Sprintf("Profile %s: unknown columns %s", profile.Name, strings.Join(unknown, ", ")), ui.MessageUrgencyWarn)
	}
	st.FileColumns = set
	st.Checked = checkedFor(st.Entries, profile.Meta)
	st.HiddenMeta = hiddenMetaNames(st.Entries, profile.Meta)
	h.setPhase(dialog.ColumnsPhaseMain)
}

// saveProfile writes the checkboxes as the named profile and returns to the main screen.
func (h *Handler) saveProfile(name, description string) {
	st := &h.model.ColumnsDialog
	profile := columnprofile.Profile{
		Name: name, Description: description, Columns: st.FileColumns.Names(), Meta: dialogMetaNames(st),
	}
	if profile.Meta == nil {
		profile.Meta = []string{}
	}
	if err := columnprofile.Upsert(h.profilesPath(), profile); err != nil {
		h.host.SetErrorMessage(columnsMessageTitle, err)
		return
	}
	h.reloadProfiles()
	h.setPhase(dialog.ColumnsPhaseMain)
	h.host.SetTransientMessage("Profile saved: "+name, ui.MessageUrgencySuccess)
}

func (h *Handler) confirmSavePrompt() {
	st := &h.model.ColumnsDialog
	name := strings.TrimSpace(st.SaveName.Value)
	switch {
	case name == "":
		h.host.SetTransientMessage("Profile name is required", ui.MessageUrgencyWarn)
		st.Focus = dialog.ColumnsSaveNameInput
	case columnprofile.IsBuiltinName(name):
		h.host.SetTransientMessage(fmt.Sprintf("%q is a built-in profile; choose another name", name), ui.MessageUrgencyWarn)
		st.Focus = dialog.ColumnsSaveNameInput
	default:
		h.saveProfile(name, strings.TrimSpace(st.SaveDesc.Value))
	}
}

// focusedPromptField is the save prompt's focused input (Name when the focus is on neither).
func (h *Handler) focusedPromptField() (dlg.ID, *dialog.ColumnsPromptField) {
	st := &h.model.ColumnsDialog
	if st.Focus == dialog.ColumnsSaveDescInput {
		return dialog.ColumnsSaveDescInput, &st.SaveDesc
	}
	return dialog.ColumnsSaveNameInput, &st.SaveName
}

func (h *Handler) handleSavePromptKey(event *tcell.EventKey) {
	st := &h.model.ColumnsDialog
	spec := h.dialogSpec()
	if ev, ok := dlg.HandleKey(spec, &st.Focus, event); ok {
		h.applySavePromptEvent(ev)
		return
	}
	if st.Focus == dlg.OK || st.Focus == dlg.Cancel {
		return
	}
	id, field := h.focusedPromptField()
	w, screenH := h.screen.Size()
	edit := scrollquery.NewEdit(&field.Value, &field.Cursor, &field.Scroll, max(dlg.InputTextWidth(w, screenH, spec, id), 10), func() {})
	scrollquery.HandleKey(h.keysDialogInput, event, true, edit)
}

func (h *Handler) applySavePromptEvent(ev dlg.Event) {
	switch {
	case ev.Kind == dlg.EventCaret:
		if ev.ID == dialog.ColumnsSaveDescInput {
			h.model.ColumnsDialog.SaveDesc.Cursor = ev.Index
		} else {
			h.model.ColumnsDialog.SaveName.Cursor = ev.Index
		}
	case ev.ID == dlg.Cancel:
		h.DialogCancel()
	case ev.ID == dlg.OK, ev.ID == dialog.ColumnsSaveNameInput, ev.ID == dialog.ColumnsSaveDescInput:
		h.confirmSavePrompt()
	}
}

// deleteSelectedProfile removes the saved profile under the load picker's cursor.
func (h *Handler) deleteSelectedProfile() {
	profile, ok := h.selectedProfile()
	if !ok {
		return
	}
	if profile.Builtin {
		h.host.SetTransientMessage(fmt.Sprintf("%q is a built-in profile and cannot be deleted", profile.Name), ui.MessageUrgencyWarn)
		return
	}
	if err := columnprofile.Remove(h.profilesPath(), profile.Name); err != nil {
		h.host.SetErrorMessage(columnsMessageTitle, err)
		return
	}
	h.reloadProfiles()
	pk := &h.model.ColumnsDialog.Picker
	pk.Items = columnprofile.All(h.profiles)
	h.syncPickerRanks()
	h.host.SetTransientMessage("Profile deleted: "+profile.Name, ui.MessageUrgencyInfo)
}

// setSelectedProfileDefault makes the profile under the load picker's cursor the startup profile.
func (h *Handler) setSelectedProfileDefault() {
	profile, ok := h.selectedProfile()
	if !ok {
		return
	}
	if err := h.host.SaveDefaultProfile(profile.Name); err != nil {
		h.host.SetErrorMessage(columnsMessageTitle, err)
		return
	}
	h.config.Panels.DefaultColumnProfile = profile.Name
	st := &h.model.ColumnsDialog
	st.DefaultProfile = profile.Name
	h.syncPickerRanks()
	h.host.SetTransientMessage("Startup columns: "+profile.Name, ui.MessageUrgencySuccess)
}
