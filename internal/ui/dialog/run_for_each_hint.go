package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func runForEachCommandErrorStyle(styles theme.Theme, dbg tcell.Color) tcell.Style {
	errFG, _, _ := styles.DialogInputActiveError.Decompose()
	return styles.DialogText.Foreground(errFG).Background(dbg)
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

func runForEachPreviewStyle(styles theme.Theme, dbg tcell.Color) tcell.Style {
	return styles.DialogText.Background(dbg)
}

// runForEachCommandFieldRows is vertical space for the command block (label, blank, input, optional error/preview).
func runForEachCommandFieldRows(state FileDialogState) int {
	rows := 4
	if runForEachShowsCommandError(state) || runForEachShowsPreview(state) {
		rows++
	}
	return rows
}
