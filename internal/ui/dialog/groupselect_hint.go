package dialog

// GroupSelectShowsFullPath reports whether the "Match full path" checkbox is shown. Find
// and output contexts only — panel entries all share one directory, so it would be
// meaningless there.
func GroupSelectShowsFullPath(state GroupSelectState) bool {
	return state.Context == "find" || state.Context == "index-find" || state.Context == "output"
}

// groupSelectPatternHintText is the compile-error hint under the Select group pattern input.
func groupSelectPatternHintText(state GroupSelectState) string {
	return patternHintText(state.Text, state.PatternMode, state.CaseSensitive, state.PatternCompileHint)
}

// groupSelectPatternInvalid reports whether the pattern input row should render in the dialog's
// error style: an uncompilable pattern, or a valid pattern that currently matches nothing to
// select/unselect.
func groupSelectPatternInvalid(state GroupSelectState) bool {
	return patternInvalid(state.Text, groupSelectPatternHintText(state), state.PreviewShow, state.PreviewFiles, state.PreviewFolders)
}
