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

// runForEachCommandFieldRows is vertical space for the command block (label, blank, input, optional error).
func runForEachCommandFieldRows(state FileDialogState) int {
	rows := 4
	if runForEachShowsCommandError(state) {
		rows++
	}
	return rows
}
