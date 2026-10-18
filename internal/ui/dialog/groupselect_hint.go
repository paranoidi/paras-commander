package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	GroupSelectFocusShellRadio  = 0
	GroupSelectFocusRegexRadio  = 1
	GroupSelectFocusSimpleRadio = 2
	// GroupSelectFocusTypeFirst..+FilterTypeCount-1 are the type checkboxes, in FilterTypeLabels
	// order.
	GroupSelectFocusTypeFirst   = 3
	GroupSelectFocusPattern     = GroupSelectFocusTypeFirst + FilterTypeCount // 8
	GroupSelectFocusFilesOnly   = GroupSelectFocusPattern + 1
	GroupSelectFocusDirsOnly    = GroupSelectFocusPattern + 2
	GroupSelectFocusCase        = GroupSelectFocusPattern + 3
	GroupSelectFocusFullPath    = GroupSelectFocusPattern + 4
	GroupSelectFocusIncludeMeta = GroupSelectFocusPattern + 5
	GroupSelectFocusOnlyMeta    = GroupSelectFocusPattern + 6
)

// GroupSelectContentCount is the number of focus slots before OK (Match full path's slot is
// always counted, even when hidden; meta slots only when shown).
func GroupSelectContentCount(metaColumnCount int) int {
	if metaColumnCount > 0 {
		return GroupSelectFocusOnlyMeta + 1
	}
	return GroupSelectFocusFullPath + 1
}

// GroupSelectShowsFullPath reports whether the "Match full path" checkbox is shown. Find
// and output contexts only — panel entries all share one directory, so it would be
// meaningless there.
func GroupSelectShowsFullPath(state GroupSelectState) bool {
	return state.Context == "find" || state.Context == "output"
}

func groupSelectPatternHintText(state GroupSelectState) string {
	if hint := strings.TrimSpace(state.PatternCompileHint); hint != "" {
		return hint
	}
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

func groupSelectShowsPatternHint(state GroupSelectState) bool {
	return groupSelectPatternHintText(state) != ""
}

func groupSelectPatternHintStyle(styles theme.Theme, dbg tcell.Color) tcell.Style {
	errFG, _, _ := styles.DialogInputActiveError.Decompose()
	return styles.DialogText.Foreground(errFG).Background(dbg)
}

// groupSelectPatternInvalid reports whether the pattern input row should render in the
// dialog's error style (mass-rename-style): an uncompilable pattern, or a valid pattern that
// currently matches nothing to select/unselect.
func groupSelectPatternInvalid(state GroupSelectState) bool {
	if strings.TrimSpace(state.Text) == "" {
		return false
	}
	if groupSelectShowsPatternHint(state) {
		return true
	}
	return state.PreviewShow && state.PreviewFiles == 0 && state.PreviewFolders == 0
}

func groupSelectDialogInnerRows(state GroupSelectState) int {
	// mode radios, separator, Pattern label, input, reserved hint row, filter
	// checkboxes, separator, blank, buttons
	rows := 3 + 1 + 1 + 1 + 1 + 2 + 1 + 1 + 1
	if state.MetaColumnCount > 0 {
		rows++
	}
	return rows
}

func groupSelectDialogHeight(state GroupSelectState, layoutHeight int) int {
	height := groupSelectDialogInnerRows(state) + 2 // top and bottom border rows
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	return height
}

// GroupSelectLastContentFocus returns the last navigable content index for the given
// meta column count and full-path visibility.
func GroupSelectLastContentFocus(metaColumnCount int, showFullPath bool) int {
	if metaColumnCount > 0 {
		return GroupSelectFocusIncludeMeta // leftmost item on the meta row
	}
	if showFullPath {
		return GroupSelectFocusFullPath
	}
	return GroupSelectFocusCase
}

// groupSelectGrid is the dialog's content as rows of focus indexes (see filterGrid): Match
// full path shows only in find context, the meta row only with meta columns.
func groupSelectGrid(metaColumnCount int, showFullPath bool) [][]int {
	grid := patternDialogGridHead(GroupSelectFocusShellRadio, GroupSelectFocusTypeFirst, GroupSelectFocusPattern)
	grid = append(grid, []int{GroupSelectFocusFilesOnly, GroupSelectFocusDirsOnly})
	if showFullPath {
		grid = append(grid, []int{GroupSelectFocusCase, GroupSelectFocusFullPath})
	} else {
		grid = append(grid, []int{GroupSelectFocusCase})
	}
	if metaColumnCount > 0 {
		grid = append(grid, []int{GroupSelectFocusIncludeMeta, GroupSelectFocusOnlyMeta})
	}
	return grid
}

// GroupSelectMoveFocus applies dialog navigation over groupSelectGrid plus the OK/Cancel row,
// and Tab/Shift+Tab jumps between groups: mode radios, type checkboxes, pattern, options,
// buttons. Find context never has meta columns, so showFullPath and metaColumnCount > 0 never
// both apply.
func GroupSelectMoveFocus(focus int, key tcell.Key, metaColumnCount int, showFullPath bool) (int, bool) {
	form := NewDialogLinearForm(GroupSelectContentCount(metaColumnCount))
	okIdx := form.OKIndex()
	if key == tcell.KeyTab || key == tcell.KeyBacktab {
		starts := []int{GroupSelectFocusShellRadio, GroupSelectFocusTypeFirst, GroupSelectFocusPattern, GroupSelectFocusFilesOnly, okIdx}
		return dialogTabSegment(starts, focus, key), true
	}
	return dialogGridMove(groupSelectGrid(metaColumnCount, showFullPath), focus, key, okIdx, form.CancelIndex(), GroupSelectLastContentFocus(metaColumnCount, showFullPath))
}
