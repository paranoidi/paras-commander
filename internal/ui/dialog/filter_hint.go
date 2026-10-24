package dialog

// PatternDialogWidth is the frame width of the Filter and Select group dialogs, shared by their
// renderers and input-width computations. It fits "( ) Regular expression" plus both type columns.
const PatternDialogWidth = 60

// FilterShowsFilesDirs reports whether the Files only / Directories only row is shown; compare
// and dedup rows are always files, so it is hidden there.
func FilterShowsFilesDirs(state FilterDialogState) bool {
	return state.Target == FilterTargetPanel
}

// filterPatternHintText is the compile-error hint under the Filter pattern input.
func filterPatternHintText(state FilterDialogState) string {
	return patternHintText(state.Text, state.PatternMode, state.CaseSensitive, "")
}

// filterPatternInvalid reports whether the pattern input row should render in the dialog's
// error style: an uncompilable pattern, or a valid pattern that currently matches nothing.
func filterPatternInvalid(state FilterDialogState) bool {
	return patternInvalid(state.Text, filterPatternHintText(state), state.PreviewShow, state.PreviewFiles, state.PreviewFolders)
}
