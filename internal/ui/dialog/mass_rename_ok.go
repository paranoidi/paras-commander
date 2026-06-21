package dialog

// FileDialogMassRenameOKEnabled reports whether the mass rename dialog OK action may run
// (preview conflicts or invalid find). OK remains focusable when this returns false.
func FileDialogMassRenameOKEnabled(state FileDialogState) bool {
	if state.DialogType != FileDialogMassRename {
		return true
	}
	if state.MassRenameMode == MassRenameModeUIExternalEditor {
		return len(state.MassRenameExternalNames) > 0 && !massRenameHasErrors(state)
	}
	if len(state.Fields) > 0 && state.Fields[0].InputInvalid {
		return false
	}
	return !massRenameHasErrors(state)
}

func massRenameHasErrors(state FileDialogState) bool {
	for _, bad := range state.MassRenamePreviewAfterError {
		if bad {
			return true
		}
	}
	return false
}
