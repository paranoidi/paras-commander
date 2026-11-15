package dialog

// FilterShowsScope reports whether the Files / Dirs / Both scope row is shown; compare and dedup rows are
// always files, so it is hidden there (and the scope stays Files).
func FilterShowsScope(state FilterDialogState) bool {
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
