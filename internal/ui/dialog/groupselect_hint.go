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

func groupSelectDialogHeight(state GroupSelectState, layoutHeight int) int {
	// 3 radios + sep + label + blank + input + optional hint + 2 checkbox rows + sep + buttons + borders.
	fixed := 3 + 1 + 1 + 1 + 1 + 2 + 1 + 2 // inner rows + top/bottom border rows
	if groupSelectShowsPatternHint(state) {
		fixed++
	}
	height := fixed
	if height > layoutHeight-2 {
		height = layoutHeight - 2
	}
	if height < 14 {
		height = 14
	}
	return height
}

// GroupSelectLastContentFocus returns the last navigable content index for the given mode.
func GroupSelectLastContentFocus(mode panel.GroupPatternMode) int {
	if mode == panel.GroupPatternRegex {
		return GroupSelectFocusDirsOnly
	}
	return GroupSelectFocusCase
}

// GroupSelectMoveFocus applies dialog navigation, including 2D checkbox layout:
// Files only and Directories only share a row (Left/Right); Case sensitive sits below Files only (Down/Up).
func GroupSelectMoveFocus(focus int, key tcell.Key, mode panel.GroupPatternMode) (int, bool) {
	form := NewDialogLinearForm(7)
	showCase := GroupSelectShowsCaseSensitive(GroupSelectState{PatternMode: mode})

	switch key {
	case tcell.KeyRight:
		if focus == GroupSelectFocusFilesOnly {
			return GroupSelectFocusDirsOnly, true
		}
		if focus == form.OKIndex() {
			return form.CancelIndex(), true
		}
		return focus, false
	case tcell.KeyLeft:
		if focus == GroupSelectFocusDirsOnly {
			return GroupSelectFocusFilesOnly, true
		}
		if focus == form.CancelIndex() {
			return form.OKIndex(), true
		}
		return focus, false
	case tcell.KeyDown, tcell.KeyTab:
		switch focus {
		case GroupSelectFocusFilesOnly:
			if showCase {
				return GroupSelectFocusCase, true
			}
			return form.OKIndex(), true
		case GroupSelectFocusDirsOnly, GroupSelectFocusCase:
			return form.OKIndex(), true
		default:
			next, ok := form.MoveFocus(focus, key)
			if !ok {
				return focus, false
			}
			return groupSelectSkipHiddenCase(next, mode, form), true
		}
	case tcell.KeyUp, tcell.KeyBacktab:
		switch focus {
		case GroupSelectFocusFilesOnly, GroupSelectFocusDirsOnly:
			return GroupSelectFocusPattern, true
		case GroupSelectFocusCase:
			return GroupSelectFocusFilesOnly, true
		case form.OKIndex(), form.CancelIndex():
			return GroupSelectLastContentFocus(mode), true
		default:
			next, ok := form.MoveFocus(focus, key)
			if !ok {
				return focus, false
			}
			return groupSelectSkipHiddenCase(next, mode, form), true
		}
	default:
		next, ok := form.MoveFocus(focus, key)
		if !ok {
			return focus, false
		}
		return groupSelectSkipHiddenCase(next, mode, form), true
	}
}

func groupSelectSkipHiddenCase(focus int, mode panel.GroupPatternMode, form DialogLinearForm) int {
	if mode == panel.GroupPatternRegex && focus == GroupSelectFocusCase {
		return GroupSelectFocusDirsOnly
	}
	return focus
}
