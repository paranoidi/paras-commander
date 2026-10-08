package dialog

import "github.com/paranoidi/paras-commander/internal/jobs"

// conflictRuleRowLabels are the condition rows of the "Conflict rules" dialog: rows 0..2 are
// jobs.ConflictRules.Time, rows 3..5 are jobs.ConflictRules.Size. Render and key handling share it.
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
	// ConflictRuleRows is the number of condition rows; focus 0..ConflictRuleRows-1 are radio rows.
	ConflictRuleRows = len(conflictRuleRowLabels)
)

// ConflictAdvancedNoCompare reports whether Skip identical is hidden (archive extract).
func (st ConflictDialogState) ConflictAdvancedNoCompare() bool {
	return st.Blocker.Conflict != nil && st.Blocker.Conflict.NoCompare
}

// ConflictAdvancedSkipIdenticalFocus is the focus index of the Skip identical checkbox, or -1 when hidden.
func (st ConflictDialogState) ConflictAdvancedSkipIdenticalFocus() int {
	if st.ConflictAdvancedNoCompare() {
		return -1
	}
	return ConflictRuleRows
}

// ConflictAdvancedAllFocus is the focus index of the Apply to all checkbox.
func (st ConflictDialogState) ConflictAdvancedAllFocus() int {
	if st.ConflictAdvancedNoCompare() {
		return ConflictRuleRows
	}
	return ConflictRuleRows + 1
}

// ConflictAdvancedForm is the focus layout: six rule rows, the checkboxes, then OK and Cancel.
// Tab jumps between the time rows, the size rows, the checkboxes and the buttons.
func (st ConflictDialogState) ConflictAdvancedForm() DialogTrailingButtonsForm {
	n := st.ConflictAdvancedAllFocus() + 1
	return NewDialogTrailingButtonsForm(n, 2).WithSegments(0, conflictRuleTimeRows, ConflictRuleRows, n)
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
	st.AdvFocus = 0
	st.AdvAll = true
	st.Rules = jobs.ConflictRules{} // every row Ask, Skip identical off
	st.AdvCol = 0 // Ask, the default selection
}

// SyncAdvCol keeps the choice cursor in its column when focus changes rows; a greyed column on
// the new row falls back to Ask, which is never greyed.
func (st *ConflictDialogState) SyncAdvCol() {
	if st.AdvFocus >= ConflictRuleRows || st.ConflictAdvancedChoiceEnabled(st.AdvFocus, st.AdvCol) {
		return
	}
	st.AdvCol = 0 // Ask
}

// AdvMoveChoice moves the choice cursor left/right on the focused row, skipping greyed choices.
func (st *ConflictDialogState) AdvMoveChoice(delta int) {
	for c := st.AdvCol + delta; c >= 0 && c < len(ConflictRuleChoices); c += delta {
		if st.ConflictAdvancedChoiceEnabled(st.AdvFocus, c) {
			st.AdvCol = c
			return
		}
	}
}

// AdvSelectChoice picks the choice under the cursor for the focused row.
func (st *ConflictDialogState) AdvSelectChoice() {
	if st.AdvFocus >= ConflictRuleRows || !st.ConflictAdvancedChoiceEnabled(st.AdvFocus, st.AdvCol) {
		return
	}
	*st.ruleSlot(st.AdvFocus) = ConflictRuleChoices[st.AdvCol].Action
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
