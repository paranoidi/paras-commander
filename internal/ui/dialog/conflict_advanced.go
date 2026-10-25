package dialog

import (
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// conflictRuleRowLabels are the condition rows of the "Conflict rules" dialog: rows 0..2 are
// jobs.ConflictRules.Time, rows 3..5 are jobs.ConflictRules.Size. View and key handling share it.
var conflictRuleRowLabels = [...]string{
	"Destination newer", "Destination older", "Same time",
	"Destination smaller", "Destination larger", "Same size",
}

// ConflictRuleChoices are the radio choices of every row, left to right.
var ConflictRuleChoices = [...]struct {
	Label  string
	Action jobs.RuleAction
}{
	{"Ask", jobs.RuleAsk},
	{"Overwrite", jobs.RuleOverwrite},
	{"Skip", jobs.RuleSkip},
	{"Keep both", jobs.RuleKeepBoth},
}

const (
	conflictRuleTimeRows = 3
	// ConflictRuleRows is the number of condition rows.
	ConflictRuleRows = len(conflictRuleRowLabels)
)

// ConflictAdvancedNoCompare reports whether Skip identical is hidden (archive extract).
func (st ConflictDialogState) ConflictAdvancedNoCompare() bool {
	return st.Blocker.Conflict != nil && st.Blocker.Conflict.NoCompare
}

func (st ConflictDialogState) ruleAction(row int) jobs.RuleAction {
	if row < conflictRuleTimeRows {
		return st.Rules.Time[row]
	}
	return st.Rules.Size[row-conflictRuleTimeRows]
}

func (st *ConflictDialogState) ruleSlot(row int) *jobs.RuleAction {
	if row < conflictRuleTimeRows {
		return &st.Rules.Time[row]
	}
	return &st.Rules.Size[row-conflictRuleTimeRows]
}

// ConflictAdvancedChoiceEnabled reports whether choice col of row may be picked; size choices
// that contradict the time rows are greyed out.
func (st ConflictDialogState) ConflictAdvancedChoiceEnabled(row, col int) bool {
	return row < conflictRuleTimeRows || st.Rules.SizeChoiceEnabled(ConflictRuleChoices[col].Action)
}

// StartAdvanced switches the dialog to the rules matrix with default rules.
func (st *ConflictDialogState) StartAdvanced() {
	st.Advanced = true
	st.AdvFocus = ConflictRuleID(0, 0) // Ask, the default selection
	st.AdvAll = true
	st.Rules = jobs.ConflictRules{} // every row Ask, Skip identical off
}

// ApplyRule picks the choice named by the rule radio id for its row (a greyed choice is ignored)
// and resets size choices that now contradict the time rows.
func (st *ConflictDialogState) ApplyRule(id dlg.ID) {
	row, col, ok := ConflictRuleCell(id)
	if !ok || row < 0 || row >= ConflictRuleRows || col < 0 || col >= len(ConflictRuleChoices) || !st.ConflictAdvancedChoiceEnabled(row, col) {
		return
	}
	*st.ruleSlot(row) = ConflictRuleChoices[col].Action
	st.Rules.Normalize()
}

// ConflictAdvancedAnswer returns the answer the rules dialog would submit.
func (st ConflictDialogState) ConflictAdvancedAnswer() jobs.BlockerAnswer {
	rules := st.Rules
	if st.ConflictAdvancedNoCompare() {
		rules.SkipIdentical = false
	}
	d := jobs.DecisionRules
	if st.AdvAll {
		d = d.All()
	}
	return jobs.BlockerAnswer{Decision: d, Rules: &rules}
}
