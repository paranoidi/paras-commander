package dialog

// FileDialogMassRenameOKEnabled reports whether the mass rename dialog OK action may run
// (preview conflicts or invalid find). OK remains focusable when this returns false.
func FileDialogMassRenameOKEnabled(state FileDialogState) bool {
	if state.DialogType != FileDialogMassRename {
		return true
	}
	if len(state.Fields) > 0 && state.Fields[0].InputInvalid {
		return false
	}
	for _, bad := range state.MassRenamePreviewAfterError {
		if bad {
			return false
		}
	}
	return true
}
