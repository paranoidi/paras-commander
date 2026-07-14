package dialog

// FileDialogHasRenamePhase reports dialog types that use RenamePhase and rename-tool sub-screens.
func FileDialogHasRenamePhase(t FileDialogType) bool {
	return t == FileDialogRename || t == FileDialogDuplicate
}
