package dialog

import (
	"strings"

	"github.com/paranoidi/paras-commander/internal/ops"
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
	_, err := ops.MassRenameCompileRegex(pat, state.MassRenameCaseFold)
	if err == nil {
		return ""
	}
	hint := ops.MassRenameRegexCompileUserMessage(err)
	if hint == "" {
		return "invalid regexp"
	}
	return hint
}

func massRenameReplacementHintText(state FileDialogState) string {
	if state.MassRenameMode != MassRenameModeUIRegex {
		return ""
	}
	return ops.MassRenameReplacementSyntaxHint
}
