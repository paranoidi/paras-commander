package dialog

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// conflictRuleRowLabels are the condition rows of the "Conflict rules" dialog: rows 0..2 are
// jobs.ConflictRules.Time, rows 3..5 are jobs.ConflictRules.Size, then Same and Differ (the
// Verify rows). View and key handling share it.
var conflictRuleRowLabels = [...]string{
	"Destination newer", "Destination older", "Same time",
	"Destination smaller", "Destination larger", "Same size",
	"Same", "Differ",
}

// ConflictRuleChoice is one radio of a rule row.
type ConflictRuleChoice struct {
	Label  string
	Action jobs.RuleAction
}

// Radio choices of each kind of row, left to right. Delete source comes last so dropping it
// for a copy keeps the other column indexes.
var (
	conflictMatrixChoices = []ConflictRuleChoice{
		{"Ask", jobs.RuleAsk}, {"Overwrite", jobs.RuleOverwrite}, {"Skip", jobs.RuleSkip}, {"Keep both", jobs.RuleKeepBoth},
	}
	conflictSameChoices = []ConflictRuleChoice{
		{"Ask", jobs.RuleAsk}, {"Overwrite", jobs.RuleOverwrite}, {"Skip", jobs.RuleSkip}, {"Delete source", jobs.RuleDeleteSource},
	}
	conflictDifferChoices = []ConflictRuleChoice{
		{"Rules", jobs.RuleFallback}, {"Ask", jobs.RuleAsk}, {"Overwrite", jobs.RuleOverwrite}, {"Skip", jobs.RuleSkip},
		{"Keep both", jobs.RuleKeepBoth}, {"Delete source", jobs.RuleDeleteSource},
	}
)

const (
	conflictRuleTimeRows = 3
	conflictRuleSizeEnd  = 6
	// ConflictRuleSameRow and ConflictRuleDifferRow are the Verify rows.
	ConflictRuleSameRow   = conflictRuleSizeEnd
	ConflictRuleDifferRow = ConflictRuleSameRow + 1
	// ConflictRuleRows is the number of condition rows.
	ConflictRuleRows = len(conflictRuleRowLabels)
)

// ConflictAdvancedNoCompare reports whether Verify is hidden (archive extract).
func (st ConflictDialogState) ConflictAdvancedNoCompare() bool {
	return st.Blocker.Conflict != nil && st.Blocker.Conflict.NoCompare
}

// conflictMove reports whether the conflict belongs to a move, which offers Delete source.
func (st ConflictDialogState) conflictMove() bool {
	return st.Blocker.Conflict != nil && st.Blocker.Conflict.Move
}

// ConflictRuleChoices returns the radio choices of rule row, without Delete source unless the
// job is a move.
func (st ConflictDialogState) ConflictRuleChoices(row int) []ConflictRuleChoice {
	choices := conflictMatrixChoices
	switch row {
	case ConflictRuleSameRow:
		choices = conflictSameChoices
	case ConflictRuleDifferRow:
		choices = conflictDifferChoices
	}
	if !st.conflictMove() && choices[len(choices)-1].Action == jobs.RuleDeleteSource {
		choices = choices[:len(choices)-1]
	}
	return choices
}

func (st ConflictDialogState) ruleAction(row int) jobs.RuleAction {
	return *(&st).ruleSlot(row)
}

func (st *ConflictDialogState) ruleSlot(row int) *jobs.RuleAction {
	switch {
	case row < conflictRuleTimeRows:
		return &st.Rules.Time[row]
	case row < conflictRuleSizeEnd:
		return &st.Rules.Size[row-conflictRuleTimeRows]
	case row == ConflictRuleSameRow:
		return &st.Rules.Same
	}
	return &st.Rules.Differ
}

// ConflictAdvancedChoiceEnabled reports whether choice col of row may be picked: the Verify rows
// only while Verify is on, the time and size rows only while some file can reach them, and size
// choices only when they do not contradict the time rows.
func (st ConflictDialogState) ConflictAdvancedChoiceEnabled(row, col int) bool {
	choices := st.ConflictRuleChoices(row)
	switch {
	case col < 0 || col >= len(choices):
		return false
	case row >= conflictRuleSizeEnd:
		return st.Rules.Verify
	case !st.Rules.TimeSizeReachable():
		return false
	}
	return row < conflictRuleTimeRows || st.Rules.SizeChoiceEnabled(choices[col].Action)
}

// StartAdvanced switches the dialog to the rules matrix with default rules.
func (st *ConflictDialogState) StartAdvanced() {
	st.Advanced = true
	st.AdvFocus = ConflictRuleID(0, 0) // Ask, the default selection
	st.AdvAll = true
	st.Rules = jobs.ConflictRules{Differ: jobs.RuleFallback} // every row Ask, Verify off
}

// ApplyRule picks the choice named by the rule radio id for its row (a greyed choice is ignored)
// and resets choices that now contradict the other rows.
func (st *ConflictDialogState) ApplyRule(id dlg.ID) {
	row, col, ok := ConflictRuleCell(id)
	if !ok || row < 0 || row >= ConflictRuleRows || !st.ConflictAdvancedChoiceEnabled(row, col) {
		return
	}
	*st.ruleSlot(row) = st.ConflictRuleChoices(row)[col].Action
	st.Rules.Normalize(st.conflictMove())
}

// ToggleVerify turns content verification on or off; the Same and Differ choices are kept.
func (st *ConflictDialogState) ToggleVerify() {
	st.Rules.Verify = !st.Rules.Verify
}

// ConflictAdvancedAnswer returns the answer the rules dialog would submit.
func (st ConflictDialogState) ConflictAdvancedAnswer() jobs.BlockerAnswer {
	rules := st.Rules
	if st.ConflictAdvancedNoCompare() {
		rules.Verify = false
	}
	d := jobs.DecisionRules
	if st.AdvAll {
		d = d.All()
	}
	return jobs.BlockerAnswer{Decision: d, Rules: &rules}
}
