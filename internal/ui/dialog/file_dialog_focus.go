package dialog

// FileDialogFocusForm returns trailing-button focus layout for a file dialog state.
// Content row count matches the OK button index (fields, radios, checkboxes, etc.).
// Tab/Backtab jump between control groups: each text input row is its own group, the
// remaining content rows (radios, checkboxes) are one group, and the buttons are the last.
// Use Down/Up to step through individual items within a group.
func FileDialogFocusForm(state FileDialogState) DialogTrailingButtonsForm {
	okIdx := fileDialogOKFocusIndex(state)
	numTrailing := 2
	if state.DialogType == FileDialogMassRename {
		numTrailing = 3
	}
	form := NewDialogTrailingButtonsForm(okIdx, numTrailing)
	if okIdx <= 0 {
		return form
	}
	inputs := fileDialogInputRowCount(state)
	if inputs < 2 {
		return form.WithSegments(0, okIdx)
	}
	segs := make([]int, 0, inputs+2)
	for i := range inputs {
		segs = append(segs, i)
	}
	if inputs < okIdx {
		segs = append(segs, inputs)
	}
	return form.WithSegments(append(segs, okIdx)...)
}

// fileDialogInputRowCount returns how many leading focus rows are text input rows.
// It is 0 where focus row i is not state.Fields[i]: the delete confirmation, the list-picker
// phases, the rename tool sub-screens, and mass rename's main screen (Find/Replace sit after
// the mode radios and massRenameMoveFocusKey drives Tab there itself). The arms mirror the
// early returns in fileDialogOKFocusIndex.
func fileDialogInputRowCount(state FileDialogState) int {
	switch {
	case state.DialogType == FileDialogDelete,
		state.DialogType == FileDialogMassRename && MassRenamePickerPhase(state.MassRenamePhase),
		state.DialogType == FileDialogMassRename && state.MassRenamePhase == MassRenamePhaseMain,
		state.DialogType == FileDialogRunForEach && state.RunForEachHistoryOpen,
		renameToolActive(state):
		return 0
	}
	return len(state.Fields)
}
