package dialog

import (
	"strings"
)

func runForEachCommandErrorText(state FileDialogState) string {
	if state.DialogType != FileDialogRunForEach {
		return ""
	}
	return strings.TrimSpace(state.RunForEachCommandError)
}

func runForEachShowsCommandError(state FileDialogState) bool {
	return runForEachCommandErrorText(state) != ""
}

// runForEachPreviewText returns the expanded command preview for the first selected item, or
// "" when there is nothing to show (dialog not open on this type, no preview, or an error is
// shown instead).
func runForEachPreviewText(state FileDialogState) string {
	if runForEachShowsCommandError(state) {
		return ""
	}
	if state.DialogType != FileDialogRunForEach {
		return ""
	}
	return strings.TrimSpace(state.RunForEachPreview)
}

func runForEachShowsPreview(state FileDialogState) bool {
	return runForEachPreviewText(state) != ""
}

// runForEachCommandFieldRows is vertical space for the command block (label, input, optional error/preview).
func runForEachCommandFieldRows(state FileDialogState) int {
	rows := 3
	if runForEachShowsCommandError(state) || runForEachShowsPreview(state) {
		rows++
	}
	return rows
}
