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
	GroupSelectFocusPattern     = 3
	GroupSelectFocusFilesOnly   = 4
	GroupSelectFocusDirsOnly    = 5
	GroupSelectFocusCase        = 6
	GroupSelectFocusIncludeMeta = 7
	GroupSelectFocusOnlyMeta    = 8
)

// GroupSelectShowsCaseSensitive reports whether the case-sensitive checkbox is shown.
func GroupSelectShowsCaseSensitive(state GroupSelectState) bool {
	return state.PatternMode != panel.GroupPatternRegex
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
	rows := 3 + 1 + 1 + 1 + 1 + 2 + 1 + 1 // mode radios, separator, Pattern label, blank, input, filter checkboxes, separator, buttons
	if groupSelectShowsPatternHint(state) {
		rows++
	}
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

// GroupSelectLastContentFocus returns the last navigable content index for the given mode and meta column count.
func GroupSelectLastContentFocus(mode panel.GroupPatternMode, metaColumnCount int) int {
	if metaColumnCount > 0 {
		return GroupSelectFocusIncludeMeta // leftmost item on the meta row
	}
	if mode == panel.GroupPatternRegex {
		return GroupSelectFocusDirsOnly
	}
	return GroupSelectFocusCase
}

// GroupSelectMoveFocus applies dialog navigation, including 2D checkbox layout:
// Files only and Directories only share a row (Left/Right); Case sensitive sits below.
// When metaColumnCount > 0, "Include meta columns" and "Only meta columns" share a row below case sensitive.
func GroupSelectMoveFocus(focus int, key tcell.Key, mode panel.GroupPatternMode, metaColumnCount int) (int, bool) {
	numContent := 7
	if metaColumnCount > 0 {
		numContent += 2 // IncludeMeta + OnlyMeta
	}
	form := NewDialogLinearForm(numContent)
	showCase := GroupSelectShowsCaseSensitive(GroupSelectState{PatternMode: mode})
	showMeta := metaColumnCount > 0

	okIdx := form.OKIndex()
	switch key {
	case tcell.KeyRight:
		if focus == GroupSelectFocusFilesOnly {
			return GroupSelectFocusDirsOnly, true
		}
		if focus == GroupSelectFocusIncludeMeta && showMeta {
			return GroupSelectFocusOnlyMeta, true
		}
		if focus == okIdx {
			return form.CancelIndex(), true
		}
		return focus, false
	case tcell.KeyLeft:
		if focus == GroupSelectFocusDirsOnly {
			return GroupSelectFocusFilesOnly, true
		}
		if focus == GroupSelectFocusOnlyMeta && showMeta {
			return GroupSelectFocusIncludeMeta, true
		}
		if focus == form.CancelIndex() {
			return okIdx, true
		}
		return focus, false
	case tcell.KeyTab:
		// Segment jumps: mode radios(0-2) → pattern(3) → filters(4+) → buttons → mode radios
		switch {
		case focus < GroupSelectFocusPattern:
			return GroupSelectFocusPattern, true
		case focus == GroupSelectFocusPattern:
			return GroupSelectFocusFilesOnly, true
		case focus < okIdx:
			return okIdx, true
		default:
			return GroupSelectFocusShellRadio, true
		}
	case tcell.KeyBacktab:
		// Reverse segment jumps
		switch {
		case focus < GroupSelectFocusPattern:
			return okIdx, true
		case focus == GroupSelectFocusPattern:
			return GroupSelectFocusShellRadio, true
		case focus < okIdx:
			return GroupSelectFocusPattern, true
		default:
			return GroupSelectFocusFilesOnly, true
		}
	case tcell.KeyDown:
		switch focus {
		case GroupSelectFocusFilesOnly:
			if showCase {
				return GroupSelectFocusCase, true
			}
			if showMeta {
				return GroupSelectFocusIncludeMeta, true
			}
			return okIdx, true
		case GroupSelectFocusDirsOnly:
			if showMeta {
				return GroupSelectFocusIncludeMeta, true
			}
			return okIdx, true
		case GroupSelectFocusCase:
			if showMeta {
				return GroupSelectFocusIncludeMeta, true
			}
			return okIdx, true
		case GroupSelectFocusIncludeMeta, GroupSelectFocusOnlyMeta:
			return okIdx, true
		default:
			next, ok := form.MoveFocus(focus, tcell.KeyDown)
			if !ok {
				return focus, false
			}
			return groupSelectSkipHiddenCase(next, mode), true
		}
	case tcell.KeyUp:
		switch focus {
		case GroupSelectFocusFilesOnly, GroupSelectFocusDirsOnly:
			return GroupSelectFocusPattern, true
		case GroupSelectFocusCase:
			return GroupSelectFocusFilesOnly, true
		case GroupSelectFocusIncludeMeta, GroupSelectFocusOnlyMeta:
			if showCase {
				return GroupSelectFocusCase, true
			}
			return GroupSelectFocusFilesOnly, true
		case okIdx, form.CancelIndex():
			return GroupSelectLastContentFocus(mode, metaColumnCount), true
		default:
			next, ok := form.MoveFocus(focus, tcell.KeyUp)
			if !ok {
				return focus, false
			}
			return groupSelectSkipHiddenCase(next, mode), true
		}
	default:
		next, ok := form.MoveFocus(focus, key)
		if !ok {
			return focus, false
		}
		return groupSelectSkipHiddenCase(next, mode), true
	}
}

func groupSelectSkipHiddenCase(focus int, mode panel.GroupPatternMode) int {
	if mode == panel.GroupPatternRegex && focus == GroupSelectFocusCase {
		return GroupSelectFocusDirsOnly
	}
	return focus
}
