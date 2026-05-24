package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/ops"
	"github.com/paranoidi/paras-commander/internal/theme"
)

// massRenamePatternHintText returns the regexp compile hint shown under the Pattern field.
func massRenamePatternHintText(state FileDialogState) string {
	if state.MassRenameMode != MassRenameModeUIRegex {
		return ""
	}
	if hint := strings.TrimSpace(state.MassRenamePatternCompileHint); hint != "" {
		return hint
	}
	if len(state.Fields) == 0 || !state.Fields[0].InputInvalid {
		return ""
	}
	pat := strings.TrimSpace(state.Fields[0].Value)
	if pat == "" {
		return ""
	}
	_, err := ops.MassRenameCompileRegex(pat)
	if err == nil {
		return ""
	}
	hint := ops.MassRenameRegexCompileUserMessage(err)
	if hint == "" {
		return "invalid regexp"
	}
	return hint
}

func massRenameShowsPatternHint(state FileDialogState) bool {
	return massRenamePatternHintText(state) != ""
}

func massRenamePatternHintStyle(styles theme.Theme, dbg tcell.Color) tcell.Style {
	errFG, _, _ := styles.DialogInputActiveError.Decompose()
	return styles.DialogText.Foreground(errFG).Background(dbg)
}
