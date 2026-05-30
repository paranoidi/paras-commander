package dialog

// FileDialogFocusForm returns trailing-button focus layout for a file dialog state.
// Content row count matches the OK button index (fields, radios, checkboxes, etc.).
func FileDialogFocusForm(state FileDialogState) DialogTrailingButtonsForm {
	return NewDialogTrailingButtonsForm(fileDialogOKFocusIndex(state), 2)
}
