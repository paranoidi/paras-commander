package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	FilterFocusShellRadio  = 0
	FilterFocusRegexRadio  = 1
	FilterFocusSimpleRadio = 2
	// FilterFocusTypeFirst..FilterFocusTypeFirst+FilterTypeCount-1 are the type checkboxes,
	// in FilterTypeLabels order.
	FilterFocusTypeFirst   = 3
	FilterFocusPattern     = FilterFocusTypeFirst + FilterTypeCount // 8
	FilterFocusFilesOnly   = FilterFocusPattern + 1
	FilterFocusDirsOnly    = FilterFocusPattern + 2
	FilterFocusInvert      = FilterFocusPattern + 3
	FilterFocusCase        = FilterFocusPattern + 4
	FilterFocusIncludeMeta = FilterFocusPattern + 5
	FilterFocusOnlyMeta    = FilterFocusPattern + 6
)

// FilterDialogWidth is the Filter dialog's frame width, shared by the renderer and the
// handler's input-width computation. It fits "( ) Regular expression" plus both type columns.
const FilterDialogWidth = 60

// FilterContentCount is the number of navigable content items (meta row included when shown).
func FilterContentCount(metaColumnCount int) int {
	if metaColumnCount > 0 {
		return FilterFocusOnlyMeta + 1
	}
	return FilterFocusCase + 1
}

// FilterShowsFilesDirs reports whether the Files only / Directories only row is shown; compare
// and dedup rows are always files, so it is hidden there.
func FilterShowsFilesDirs(state FilterDialogState) bool {
	return state.Target == FilterTargetPanel
}

func filterPatternHintText(state FilterDialogState) string {
	if state.PatternMode != panel.GroupPatternRegex && state.PatternMode != panel.GroupPatternShell {
		return ""
	}
	pat := strings.TrimSpace(state.Text)
	if pat == "" {
		return ""
	}
	_, err := panel.NewGroupMatcher(pat, state.PatternMode, state.CaseSensitive)
	if err == nil {
		return ""
	}
	msg := err.Error()
	const regexPrefix = "invalid regexp: "
	const shellPrefix = "invalid shell pattern: "
	if strings.HasPrefix(msg, regexPrefix) {
		msg = strings.TrimPrefix(msg, regexPrefix)
	} else if strings.HasPrefix(msg, shellPrefix) {
		msg = strings.TrimPrefix(msg, shellPrefix)
	}
	if msg == "" {
		if state.PatternMode == panel.GroupPatternRegex {
			return "invalid regexp"
		}
		return "invalid shell pattern"
	}
	return msg
}

func filterShowsPatternHint(state FilterDialogState) bool {
	return filterPatternHintText(state) != ""
}

func filterPatternHintStyle(styles theme.Theme, dbg tcell.Color) tcell.Style {
	errFG, _, _ := styles.DialogInputActiveError.Decompose()
	return styles.DialogText.Foreground(errFG).Background(dbg)
}

// filterPatternInvalid reports whether the pattern input row should render in the dialog's
// error style: an uncompilable pattern, or a valid pattern that currently matches nothing.
func filterPatternInvalid(state FilterDialogState) bool {
	if strings.TrimSpace(state.Text) == "" {
		return false
	}
	if filterShowsPatternHint(state) {
		return true
	}
	return state.PreviewShow && state.PreviewFiles == 0 && state.PreviewFolders == 0
}

func filterDialogInnerRows(state FilterDialogState) int {
	// mode radios, separator, Pattern label, input, reserved hint row, filter checkbox rows,
	// separator, blank, buttons
	rows := 3 + 1 + 1 + 1 + 1 + 2 + 1 + 1 + 1
	if !FilterShowsFilesDirs(state) {
		rows--
	}
	if state.MetaColumnCount > 0 {
		rows++
	}
	return rows
}

func filterDialogHeight(state FilterDialogState, layoutHeight int) int {
	height := filterDialogInnerRows(state) + 2 // top and bottom border rows
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	return height
}

// filterGrid is the dialog's content as rows of focus indexes (-1 = empty cell), hidden cells
// (case in regex mode, meta row without meta columns) omitted. Up/Down keep the column, Left/Right
// move within a row.
func filterGrid(state FilterDialogState) [][]int {
	t := FilterFocusTypeFirst
	grid := [][]int{
		{FilterFocusShellRadio, t, t + 3},
		{FilterFocusRegexRadio, t + 1, t + 4},
		{FilterFocusSimpleRadio, t + 2, -1},
		{FilterFocusPattern},
	}
	if FilterShowsFilesDirs(state) {
		grid = append(grid, []int{FilterFocusFilesOnly, FilterFocusDirsOnly})
	}
	grid = append(grid, []int{FilterFocusInvert, FilterFocusCase})
	if state.MetaColumnCount > 0 {
		grid = append(grid, []int{FilterFocusIncludeMeta, FilterFocusOnlyMeta})
	}
	return grid
}

func filterGridFind(grid [][]int, focus int) (row, col int) {
	for r, cells := range grid {
		for c, f := range cells {
			if f == focus {
				return r, c
			}
		}
	}
	return -1, -1
}

// filterGridCell returns the cell at (row, col), falling back to the nearest non-empty cell to
// the left when col is past the row's end or empty.
func filterGridCell(grid [][]int, row, col int) int {
	cells := grid[row]
	if col >= len(cells) {
		col = len(cells) - 1
	}
	for ; col > 0 && cells[col] < 0; col-- {
	}
	return cells[col]
}

// FilterMoveFocus applies dialog navigation over filterGrid plus the OK/Cancel button row, and
// Tab/Shift+Tab jumps between groups: mode radios, type checkboxes, pattern, options, buttons.
func FilterMoveFocus(focus int, key tcell.Key, state FilterDialogState) (int, bool) {
	form := NewDialogLinearForm(FilterContentCount(state.MetaColumnCount))
	okIdx, cancelIdx := form.OKIndex(), form.CancelIndex()
	grid := filterGrid(state)
	optionsStart := FilterFocusFilesOnly
	if !FilterShowsFilesDirs(state) {
		optionsStart = FilterFocusInvert
	}

	switch key {
	case tcell.KeyTab, tcell.KeyBacktab:
		starts := []int{FilterFocusShellRadio, FilterFocusTypeFirst, FilterFocusPattern, optionsStart, okIdx}
		seg := 0
		for i, st := range starts {
			if focus >= st {
				seg = i
			}
		}
		if key == tcell.KeyTab {
			return starts[(seg+1)%len(starts)], true
		}
		return starts[(seg+len(starts)-1)%len(starts)], true
	}

	if focus == okIdx || focus == cancelIdx {
		switch key {
		case tcell.KeyLeft:
			if focus == cancelIdx {
				return okIdx, true
			}
		case tcell.KeyRight:
			if focus == okIdx {
				return cancelIdx, true
			}
		case tcell.KeyUp:
			return grid[len(grid)-1][0], true
		}
		return focus, false
	}

	row, col := filterGridFind(grid, focus)
	if row < 0 {
		return focus, false
	}
	switch key {
	case tcell.KeyLeft:
		if col > 0 {
			return grid[row][col-1], true
		}
	case tcell.KeyRight:
		if col+1 < len(grid[row]) && grid[row][col+1] >= 0 {
			return grid[row][col+1], true
		}
	case tcell.KeyUp:
		if row > 0 {
			return filterGridCell(grid, row-1, col), true
		}
	case tcell.KeyDown:
		if row+1 < len(grid) {
			return filterGridCell(grid, row+1, col), true
		}
		return okIdx, true
	}
	return focus, false
}
