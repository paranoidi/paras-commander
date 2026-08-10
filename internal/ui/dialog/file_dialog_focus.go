package dialog

// FileDialogFocusForm returns trailing-button focus layout for a file dialog state.
// Content row count matches the OK button index (fields, radios, checkboxes, etc.).
// Tab/Backtab jump between the content segment and the buttons segment (skipping
// individual items); use Down/Up to step through individual items within a segment.
func FileDialogFocusForm(state FileDialogState) DialogTrailingButtonsForm {
	okIdx := fileDialogOKFocusIndex(state)
	numTrailing := 2
	if state.DialogType == FileDialogMassRename {
		numTrailing = 3
	}
	form := NewDialogTrailingButtonsForm(okIdx, numTrailing)
	if okIdx > 0 {
		form = form.WithSegments(0, okIdx)
	}
	return form
}
