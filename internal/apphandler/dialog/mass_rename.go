package dialog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
)

// OpenMassRenameDialog opens the mass-rename dialog for p's selection.
func (h *Handler) OpenMassRenameDialog(p *panel.State) {
	src, err := ops.ResolveSource(p)
	if err != nil {
		h.host.SetErrorMessage("Mass rename", err)
		return
	}
	if len(src.Entries) == 0 {
		h.host.SetErrorMessage("Mass rename", fmt.Errorf("no files selected"))
		return
	}
	sources := make([]dialog.MassRenameSource, 0, len(src.Entries))
	for _, e := range src.Entries {
		ap := e.Path
		if !filepath.IsAbs(ap) {
			ap = filepath.Join(p.PathString(), ap)
		}
		ap = filepath.Clean(ap)
		sources = append(sources, dialog.MassRenameSource{Path: ap, Name: filepath.Base(ap)})
	}
	fields := []dialog.FileDialogField{
		{Label: "Find", Value: "", Cursor: 0},
		{Label: "Replace", Value: "", Cursor: 0},
	}
	h.model.FileDialog = dialog.FileDialogState{
		Open:                       true,
		DialogType:                 dialog.FileDialogMassRename,
		Fields:                     fields,
		FocusedField:               dialog.MassRenameFindFieldFocus,
		MassRenameMode:             dialog.MassRenameModeUISimple,
		MassRenameCaseFold:         true,
		MassRenameStripSpaces:      true,
		MassRenameShowOnlyModified: false,
		MassRenamePreviewScroll:    0,
		MassRenameSources:          sources,
		MassRenamePreviewBefore:    nil,
		MassRenamePreviewAfter:     nil,
	}
	h.MassRenameSyncFieldLabels()
	h.RecomputeMassRenamePreview()
}

// MassRenameSyncFieldLabels updates the two visible field labels (Find/Replace vs
// Pattern/Replacement) to match the current mode; no-op in External-editor mode.
func (h *Handler) MassRenameSyncFieldLabels() {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename || len(d.Fields) < 2 {
		return
	}
	if d.MassRenameMode == dialog.MassRenameModeUIExternalEditor || d.MassRenameMode == dialog.MassRenameModeUICapitalize {
		return // no fields visible in external editor / capitalize mode
	}
	if d.MassRenameMode == dialog.MassRenameModeUISimple {
		d.Fields[0].Label = "Find"
		d.Fields[1].Label = "Replace"
	} else {
		d.Fields[0].Label = "Pattern"
		d.Fields[1].Label = "Replacement"
	}
}

// RecomputeMassRenamePreview recomputes the before/after preview rows from the current
// find/replace (or external-editor names) and dialog options.
func (h *Handler) RecomputeMassRenamePreview() {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return
	}
	d.Message = ""
	d.MassRenamePatternCompileHint = ""
	d.MassRenameReplacementSyntaxHint = ""
	if len(d.Fields) > 0 {
		d.Fields[0].InputInvalid = false
	}
	if len(d.MassRenameSources) == 0 {
		d.MassRenamePreviewBefore = nil
		d.MassRenamePreviewAfter = nil
		d.MassRenamePreviewBeforeRemoved = nil
		d.MassRenamePreviewBeforeReplaced = nil
		d.MassRenamePreviewAfterAdded = nil
		return
	}

	if d.MassRenameMode == dialog.MassRenameModeUIExternalEditor {
		h.recomputeMassRenameExternalEditorPreview()
		return
	}
	if d.MassRenameMode == dialog.MassRenameModeUICapitalize {
		h.recomputeMassRenameCapitalizePreview()
		return
	}

	entries := make([]localfs.Entry, len(d.MassRenameSources))
	for i, s := range d.MassRenameSources {
		entries[i] = localfs.Entry{Name: s.Name, Path: s.Path, Type: localfs.EntryFile}
	}
	panelPath := h.host.ActivePanel().PathString()
	find, replace := "", ""
	if len(d.Fields) > 0 {
		find = d.Fields[0].Value
	}
	if len(d.Fields) > 1 {
		replace = d.Fields[1].Value
	}
	mode := ops.MassRenameModeSimple
	caseFold := d.MassRenameCaseFold
	var rx *regexp.Regexp
	if d.MassRenameMode == dialog.MassRenameModeUIRegex {
		mode = ops.MassRenameModeRegex
		if strings.TrimSpace(find) == "" {
			rx = nil
		} else {
			var err error
			rx, err = ops.MassRenameCompileRegex(find, caseFold)
			if err != nil {
				if len(d.Fields) > 0 {
					d.Fields[0].InputInvalid = true
				}
				d.MassRenamePatternCompileHint = ops.MassRenameRegexCompileUserMessage(err)
				if strings.TrimSpace(d.MassRenamePatternCompileHint) == "" {
					d.MassRenamePatternCompileHint = strings.TrimSpace(err.Error())
				}
				rx = nil // preview as no-op transform; error is shown on the pattern field only
			}
		}
	}
	d.MassRenameReplacementSyntaxHint = ops.MassRenameReplacementSyntaxHint(rx)
	rows, err := ops.MassRenameCompute(entries, panelPath, mode, find, replace, caseFold, d.MassRenameStripSpaces, rx)
	if err != nil {
		d.Message = err.Error()
		d.MassRenamePreviewBefore = []string{"! " + err.Error()}
		d.MassRenamePreviewAfter = []string{""}
		d.MassRenamePreviewBeforeRemoved = nil
		d.MassRenamePreviewBeforeReplaced = nil
		d.MassRenamePreviewAfterAdded = nil
		return
	}
	if len(d.Fields) > 0 && !d.Fields[0].InputInvalid && !ops.MassRenameFindMatchesAny(rows, mode, find, caseFold, rx) {
		d.Fields[0].InputInvalid = true
	}
	rowErrs := ops.MassRenameRowErrors(rows)
	before := make([]string, 0, len(rows))
	after := make([]string, 0, len(rows))
	beforeRemoved := make([][]search.Range, 0, len(rows))
	beforeReplaced := make([][]search.Range, 0, len(rows))
	afterAdded := make([][]search.Range, 0, len(rows))
	afterError := make([]bool, 0, len(rows))
	for i, r := range rows {
		if d.MassRenameShowOnlyModified && r.OldBase == r.NewBase {
			continue
		}
		matchRanges := ops.MassRenameMatchRanges(r.OldBase, mode, find, caseFold, rx)
		removed, replaced := ops.MassRenameBeforePreviewHighlightRanges(matchRanges, replace)
		before = append(before, r.OldBase)
		after = append(after, r.NewBase)
		beforeRemoved = append(beforeRemoved, removed)
		beforeReplaced = append(beforeReplaced, replaced)
		afterAdded = append(afterAdded, ops.MassRenameReplacementRanges(r.OldBase, mode, find, replace, caseFold, rx))
		afterError = append(afterError, i < len(rowErrs) && rowErrs[i] != nil)
	}
	d.MassRenamePreviewBefore = before
	d.MassRenamePreviewAfter = after
	d.MassRenamePreviewBeforeRemoved = beforeRemoved
	d.MassRenamePreviewBeforeReplaced = beforeReplaced
	d.MassRenamePreviewAfterAdded = afterAdded
	d.MassRenamePreviewAfterError = afterError
	_, height := h.screen.Size()
	vp := dialog.MassRenamePreviewViewportRows(height, *d)
	dialog.MassRenameEnsurePreviewScroll(&h.model.FileDialog, vp, len(before))
}

func (h *Handler) recomputeMassRenameExternalEditorPreview() {
	d := &h.model.FileDialog
	namesReady := len(d.MassRenameExternalNames) == len(d.MassRenameSources)
	var rowErrs []error
	if namesReady {
		rows := make([]ops.MassRenameRow, len(d.MassRenameSources))
		for i, src := range d.MassRenameSources {
			newBase := d.MassRenameExternalNames[i]
			if d.MassRenameStripSpaces {
				newBase = strings.TrimSpace(newBase)
			}
			rows[i] = ops.MassRenameRow{SourcePath: src.Path, OldBase: src.Name, NewBase: newBase}
		}
		rowErrs = ops.MassRenameRowErrors(rows)
	}
	before := make([]string, 0, len(d.MassRenameSources))
	after := make([]string, 0, len(d.MassRenameSources))
	beforeRemoved := make([][]search.Range, 0, len(d.MassRenameSources))
	beforeReplaced := make([][]search.Range, 0, len(d.MassRenameSources))
	afterAdded := make([][]search.Range, 0, len(d.MassRenameSources))
	afterError := make([]bool, 0, len(d.MassRenameSources))
	for i, src := range d.MassRenameSources {
		newBase := src.Name
		if namesReady {
			newBase = d.MassRenameExternalNames[i]
			if d.MassRenameStripSpaces {
				newBase = strings.TrimSpace(newBase)
			}
		}
		if d.MassRenameShowOnlyModified && src.Name == newBase {
			continue
		}
		before = append(before, src.Name)
		after = append(after, newBase)
		beforeRemoved = append(beforeRemoved, nil)
		beforeReplaced = append(beforeReplaced, nil)
		if namesReady {
			_, added := dialog.MassRenameDiff(src.Name, newBase)
			afterAdded = append(afterAdded, added)
		} else {
			afterAdded = append(afterAdded, nil)
		}
		afterError = append(afterError, i < len(rowErrs) && rowErrs[i] != nil)
	}
	d.MassRenamePreviewBefore = before
	d.MassRenamePreviewAfter = after
	d.MassRenamePreviewBeforeRemoved = beforeRemoved
	d.MassRenamePreviewBeforeReplaced = beforeReplaced
	d.MassRenamePreviewAfterAdded = afterAdded
	d.MassRenamePreviewAfterError = afterError
	_, height := h.screen.Size()
	vp := dialog.MassRenamePreviewViewportRows(height, *d)
	dialog.MassRenameEnsurePreviewScroll(d, vp, len(before))
}

// recomputeMassRenameCapitalizePreview recomputes the before/after preview rows for
// Capitalize mode. Mirrors recomputeMassRenameExternalEditorPreview's structure: no
// find/regex match ranges exist in this mode, so before-column highlights are always nil
// and after-column highlights come from dialog.MassRenameDiff like the external-editor path.
func (h *Handler) recomputeMassRenameCapitalizePreview() {
	d := &h.model.FileDialog
	entries := make([]localfs.Entry, len(d.MassRenameSources))
	for i, s := range d.MassRenameSources {
		entries[i] = localfs.Entry{Name: s.Name, Path: s.Path, Type: localfs.EntryFile}
	}
	panelPath := h.host.ActivePanel().PathString()
	rows, err := ops.MassRenameComputeCapitalize(entries, panelPath, d.MassRenameCapEachWord, d.MassRenameCapPunctSep, d.MassRenameStripSpaces)
	if err != nil {
		d.Message = err.Error()
		d.MassRenamePreviewBefore = []string{"! " + err.Error()}
		d.MassRenamePreviewAfter = []string{""}
		d.MassRenamePreviewBeforeRemoved = nil
		d.MassRenamePreviewBeforeReplaced = nil
		d.MassRenamePreviewAfterAdded = nil
		return
	}
	rowErrs := ops.MassRenameRowErrors(rows)
	before := make([]string, 0, len(rows))
	after := make([]string, 0, len(rows))
	beforeRemoved := make([][]search.Range, 0, len(rows))
	beforeReplaced := make([][]search.Range, 0, len(rows))
	afterAdded := make([][]search.Range, 0, len(rows))
	afterError := make([]bool, 0, len(rows))
	for i, r := range rows {
		if d.MassRenameShowOnlyModified && r.OldBase == r.NewBase {
			continue
		}
		before = append(before, r.OldBase)
		after = append(after, r.NewBase)
		beforeRemoved = append(beforeRemoved, nil)
		beforeReplaced = append(beforeReplaced, nil)
		_, added := dialog.MassRenameDiff(r.OldBase, r.NewBase)
		afterAdded = append(afterAdded, added)
		afterError = append(afterError, i < len(rowErrs) && rowErrs[i] != nil)
	}
	d.MassRenamePreviewBefore = before
	d.MassRenamePreviewAfter = after
	d.MassRenamePreviewBeforeRemoved = beforeRemoved
	d.MassRenamePreviewBeforeReplaced = beforeReplaced
	d.MassRenamePreviewAfterAdded = afterAdded
	d.MassRenamePreviewAfterError = afterError
	_, height := h.screen.Size()
	vp := dialog.MassRenamePreviewViewportRows(height, *d)
	dialog.MassRenameEnsurePreviewScroll(d, vp, len(before))
}

// ApplyMassRenameModeFromFocus sets MassRenameMode from the currently focused mode radio and
// recomputes labels/preview.
func (h *Handler) ApplyMassRenameModeFromFocus() {
	d := &h.model.FileDialog
	switch d.FocusedField {
	case 0:
		h.massRenameSwitchMode(d, dialog.MassRenameModeUISimple)
	case 1:
		h.massRenameSwitchMode(d, dialog.MassRenameModeUIRegex)
	case 2:
		h.massRenameSwitchMode(d, dialog.MassRenameModeUIExternalEditor)
	case 3:
		h.massRenameSwitchMode(d, dialog.MassRenameModeUICapitalize)
	}
}

// massRenameSwitchMode sets the mass-rename mode, snaps focus onto mode's own radio when
// focus is currently on a different radio row, and refreshes clamp/labels/preview. Shared by
// ApplyMassRenameModeFromFocus and the Alt-letter mode shortcuts (handleMassRenameAltShortcut).
func (h *Handler) massRenameSwitchMode(d *dialog.FileDialogState, mode dialog.MassRenameModeUI) {
	prev := d.MassRenameMode
	d.MassRenameMode = mode
	if d.FocusedField < 4 && d.FocusedField != dialog.MassRenameModeRadioFocus(mode) {
		d.FocusedField = dialog.MassRenameModeRadioFocus(mode)
	}
	h.MassRenameClampFocusAfterModeChange(prev)
	h.MassRenameSyncFieldLabels()
	h.RecomputeMassRenamePreview()
}

// MassRenameClampFocusAfterModeChange keeps FocusedField valid when switching modes. prev is
// the mode before the switch. Driven by the FocusIdx helpers rather than literal indices so it
// generalizes across all four modes: focus on a checkbox row that exists (at some index) in
// both prev and the new mode follows that checkbox to its new index; focus on a row that only
// existed in prev (Find/Replace, or a stale Capitalize-only checkbox row) lands on the new
// mode's "Show only modified" checkbox.
func (h *Handler) MassRenameClampFocusAfterModeChange(prev dialog.MassRenameModeUI) {
	d := &h.model.FileDialog
	if d.MassRenameMode == prev {
		return
	}
	if d.FocusedField < 4 {
		return // on a radio row: nothing to clamp
	}
	prevHasFields := prev == dialog.MassRenameModeUISimple || prev == dialog.MassRenameModeUIRegex
	newHasFields := d.MassRenameMode == dialog.MassRenameModeUISimple || d.MassRenameMode == dialog.MassRenameModeUIRegex
	if prevHasFields && newHasFields {
		return // Simple <-> Regex share identical Find/Replace/checkbox indices: no remap needed.
	}
	prevState := *d
	prevState.MassRenameMode = prev
	switch {
	case d.FocusedField == dialog.MassRenameShowModifiedFocusIdx(prevState):
		d.FocusedField = dialog.MassRenameShowModifiedFocusIdx(*d)
	case d.FocusedField == dialog.MassRenameStripFocusIdx(prevState):
		d.FocusedField = dialog.MassRenameStripFocusIdx(*d)
	case dialog.MassRenameCaseFocusIdx(prevState) >= 0 && d.FocusedField == dialog.MassRenameCaseFocusIdx(prevState):
		if idx := dialog.MassRenameCaseFocusIdx(*d); idx >= 0 {
			d.FocusedField = idx
		} else {
			d.FocusedField = dialog.MassRenameShowModifiedFocusIdx(*d)
		}
	case d.FocusedField == dialog.MassRenameFindFieldFocus || d.FocusedField == dialog.MassRenameFindFieldFocus+1:
		// Leaving Simple/Regex's Find/Replace fields (no idx-helper exists for these).
		d.FocusedField = dialog.MassRenameShowModifiedFocusIdx(*d)
	default:
		// Stale Capitalize-only checkbox index (or a stale button index): fall back to the
		// new mode's "Show only modified" checkbox.
		d.FocusedField = dialog.MassRenameShowModifiedFocusIdx(*d)
	}
}

func (h *Handler) tryRejectMassRenameOK(d *dialog.FileDialogState) bool {
	if d.DialogType != dialog.FileDialogMassRename || dialog.FileDialogMassRenameOKEnabled(*d) {
		return false
	}
	msg := h.massRenameOKBlockedMessage(d)
	if msg == "" {
		msg = "Mass rename cannot be applied"
	}
	h.host.SetTransientMessage(msg, ui.MessageUrgencyCritical)
	return true
}

func (h *Handler) massRenameOKBlockedMessage(d *dialog.FileDialogState) string {
	if d.MassRenameMode == dialog.MassRenameModeUIExternalEditor {
		if len(d.MassRenameExternalNames) == 0 {
			return "Launch the editor first (Enter on External $EDITOR option)"
		}
	} else if len(d.Fields) > 0 && d.Fields[0].InputInvalid {
		find := d.Fields[0].Value
		if d.MassRenameMode == dialog.MassRenameModeUIRegex {
			if _, err := ops.MassRenameCompileRegex(find, d.MassRenameCaseFold); err != nil {
				msg := ops.MassRenameRegexCompileUserMessage(err)
				if msg != "" {
					return msg
				}
				return strings.TrimSpace(err.Error())
			}
			if hint := strings.TrimSpace(d.MassRenamePatternCompileHint); hint != "" {
				return hint
			}
		}
		return "No selected file names match"
	}
	rows, err := h.massRenameComputeRows(d)
	if err != nil {
		return err.Error()
	}
	if vErr := ops.MassRenameValidateRows(rows); vErr != nil {
		return vErr.Error()
	}
	return ""
}

func (h *Handler) massRenameComputeRows(d *dialog.FileDialogState) ([]ops.MassRenameRow, error) {
	if len(d.MassRenameSources) == 0 {
		return nil, fmt.Errorf("no files to rename")
	}
	if d.MassRenameMode == dialog.MassRenameModeUIExternalEditor {
		if len(d.MassRenameExternalNames) == 0 {
			return nil, fmt.Errorf("launch the editor first (Enter on External $EDITOR)")
		}
		rows := make([]ops.MassRenameRow, len(d.MassRenameSources))
		for i, src := range d.MassRenameSources {
			newBase := src.Name
			if i < len(d.MassRenameExternalNames) {
				newBase = d.MassRenameExternalNames[i]
			}
			if d.MassRenameStripSpaces {
				newBase = strings.TrimSpace(newBase)
			}
			rows[i] = ops.MassRenameRow{SourcePath: src.Path, OldBase: src.Name, NewBase: newBase}
		}
		return rows, nil
	}
	entries := make([]localfs.Entry, len(d.MassRenameSources))
	for i, s := range d.MassRenameSources {
		entries[i] = localfs.Entry{Name: s.Name, Path: s.Path, Type: localfs.EntryFile}
	}
	panelPath := h.host.ActivePanel().PathString()
	if d.MassRenameMode == dialog.MassRenameModeUICapitalize {
		return ops.MassRenameComputeCapitalize(entries, panelPath, d.MassRenameCapEachWord, d.MassRenameCapPunctSep, d.MassRenameStripSpaces)
	}
	find, replace := "", ""
	if len(d.Fields) > 0 {
		find = d.Fields[0].Value
	}
	if len(d.Fields) > 1 {
		replace = d.Fields[1].Value
	}
	mode := ops.MassRenameModeSimple
	caseFold := d.MassRenameCaseFold
	var rx *regexp.Regexp
	if d.MassRenameMode == dialog.MassRenameModeUIRegex {
		mode = ops.MassRenameModeRegex
		if strings.TrimSpace(find) != "" {
			var err error
			rx, err = ops.MassRenameCompileRegex(find, caseFold)
			if err != nil {
				return nil, err
			}
		}
	}
	return ops.MassRenameCompute(entries, panelPath, mode, find, replace, caseFold, d.MassRenameStripSpaces, rx)
}

// ExecuteMassRename runs the mass-rename dialog's OK action: validates the computed rows and,
// if clean, renames every source and refreshes the active panel's selection/marks.
func (h *Handler) ExecuteMassRename() {
	d := &h.model.FileDialog
	h.RecomputeMassRenamePreview()
	if h.tryRejectMassRenameOK(d) {
		return
	}
	if len(d.MassRenameSources) == 0 {
		h.CloseFileDialog()
		return
	}
	rows, err := h.massRenameComputeRows(d)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	if vErr := ops.MassRenameValidateRows(rows); vErr != nil {
		h.host.SetTransientMessage(vErr.Error(), ui.MessageUrgencyWarn)
		return
	}
	if !ops.MassRenameHasWork(rows) {
		h.host.SetTransientMessage("Nothing to rename", ui.MessageUrgencyInfo)
		return
	}
	if err := ops.ExecuteMassRename(rows); err != nil {
		h.host.SetErrorMessage("Mass rename failed", err)
		h.CloseFileDialog()
		return
	}
	histFind, histReplace := "", ""
	if len(d.Fields) > 0 {
		histFind = d.Fields[0].Value
	}
	if len(d.Fields) > 1 {
		histReplace = d.Fields[1].Value
	}
	h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", histFind, histReplace))
	n := 0
	var renamedNames []string
	for _, r := range rows {
		if r.NewBase != r.OldBase {
			n++
			renamedNames = append(renamedNames, r.NewBase)
		}
	}
	panelDir := h.host.ActivePanel().Path
	h.CloseFileDialog()
	h.RefreshBothPanels()
	h.host.ActivePanel().AddRenameMarks(panelDir, renamedNames)
	h.host.ActivePanel().ClearSelection()
	h.host.SetTransientMessage(fmt.Sprintf("Renamed %d file(s)", n), ui.MessageUrgencyInfo)
}

// ApplyMassRenameKeepOpen runs the mass-rename dialog's Apply action: like
// ExecuteMassRename, but keeps the dialog open, re-selects the renamed batch
// under its new names, and clears the Find/Replace (or Pattern/Replacement)
// inputs so another pass can be queued immediately.
func (h *Handler) ApplyMassRenameKeepOpen() {
	d := &h.model.FileDialog
	h.RecomputeMassRenamePreview()
	if h.tryRejectMassRenameOK(d) {
		return
	}
	if len(d.MassRenameSources) == 0 {
		return
	}
	rows, err := h.massRenameComputeRows(d)
	if err != nil {
		h.host.SetTransientMessage(err.Error(), ui.MessageUrgencyWarn)
		return
	}
	if vErr := ops.MassRenameValidateRows(rows); vErr != nil {
		h.host.SetTransientMessage(vErr.Error(), ui.MessageUrgencyWarn)
		return
	}
	if !ops.MassRenameHasWork(rows) {
		h.host.SetTransientMessage("Nothing to rename", ui.MessageUrgencyInfo)
		return
	}
	if err := ops.ExecuteMassRename(rows); err != nil {
		h.host.SetErrorMessage("Mass rename failed", err)
		return
	}
	histFind, histReplace := "", ""
	if len(d.Fields) > 0 {
		histFind = d.Fields[0].Value
	}
	if len(d.Fields) > 1 {
		histReplace = d.Fields[1].Value
	}
	h.recordMassRenameHistory(h.massRenameCurrentPattern("", "", histFind, histReplace))

	n := 0
	newSources := make([]dialog.MassRenameSource, len(rows))
	newPaths := make([]string, len(rows))
	var renamedNames []string
	for i, r := range rows {
		if r.NewBase != r.OldBase {
			n++
			renamedNames = append(renamedNames, r.NewBase)
		}
		newPath := filepath.Join(filepath.Dir(r.SourcePath), r.NewBase)
		newSources[i] = dialog.MassRenameSource{Path: newPath, Name: r.NewBase}
		newPaths[i] = newPath
	}

	panelDir := h.host.ActivePanel().Path
	h.RefreshBothPanels()
	h.host.ActivePanel().AddRenameMarks(panelDir, renamedNames)
	h.host.ActivePanel().ClearSelection()
	for _, np := range newPaths {
		h.host.ActivePanel().AddSelection(np)
	}

	d.MassRenameSources = newSources
	d.Fields[0].Clear()
	d.Fields[1].Clear()
	d.MassRenameExternalNames = nil
	d.FocusedField = dialog.MassRenameFindFieldFocus
	h.RecomputeMassRenamePreview()
	h.host.SetTransientMessage(fmt.Sprintf("Renamed %d file(s)", n), ui.MessageUrgencyInfo)
}

// MassRenameEditorFooterEligible reports whether the F4 "Editor" footer hint should show:
// the mass-rename dialog is open on its main screen (in any mode), not the save/load-pattern
// sub-screens.
func (h *Handler) MassRenameEditorFooterEligible() bool {
	d := &h.model.FileDialog
	return d.Open && d.DialogType == dialog.FileDialogMassRename && d.MassRenamePhase == dialog.MassRenamePhaseMain
}

// LaunchMassRenameExternalEditor writes the current source names to a temp file, launches the
// user's $EDITOR on it (releasing the terminal), and reads the edited names back into
// MassRenameExternalNames for the External-editor mode preview.
func (h *Handler) LaunchMassRenameExternalEditor() {
	d := &h.model.FileDialog
	if d.DialogType != dialog.FileDialogMassRename {
		return
	}
	d.MassRenameMode = dialog.MassRenameModeUIExternalEditor
	h.MassRenameSyncFieldLabels()

	// Write source names to a temp file, one per line.
	tmp, err := os.CreateTemp("", "paras-rename-*.txt")
	if err != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Cannot create temp file: %v", err), ui.MessageUrgencyCritical)
		return
	}
	tmpPath := tmp.Name()
	for i, src := range d.MassRenameSources {
		line := src.Name
		if i < len(d.MassRenameSources)-1 {
			line += "\n"
		}
		if _, werr := tmp.WriteString(line); werr != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
			h.host.SetTransientMessage(fmt.Sprintf("Cannot write temp file: %v", werr), ui.MessageUrgencyCritical)
			return
		}
	}
	// Ensure last name also has a newline so editors don't complain.
	if len(d.MassRenameSources) > 0 {
		_, _ = tmp.WriteString("\n")
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		h.host.SetTransientMessage(fmt.Sprintf("Cannot write temp file: %v", err), ui.MessageUrgencyCritical)
		return
	}
	defer func() { _ = os.Remove(tmpPath) }()

	if launchErr := h.host.OpenFileInExternalEditor(tmpPath); launchErr != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Editor error: %v", launchErr), ui.MessageUrgencyCritical)
		return
	}

	raw, readErr := os.ReadFile(tmpPath)
	if readErr != nil {
		h.host.SetTransientMessage(fmt.Sprintf("Cannot read temp file: %v", readErr), ui.MessageUrgencyCritical)
		return
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	// Remove trailing empty element that Split produces when the file ends with a newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) != len(d.MassRenameSources) {
		d.MassRenameExternalNames = nil
		h.host.SetTransientMessage(
			fmt.Sprintf("Line count mismatch: got %d, expected %d", len(lines), len(d.MassRenameSources)),
			ui.MessageUrgencyCritical,
		)
		h.RecomputeMassRenamePreview()
		return
	}
	d.MassRenameExternalNames = lines
	h.RecomputeMassRenamePreview()
}
