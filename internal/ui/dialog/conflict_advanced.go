package dialog

import "github.com/paranoidi/paras-commander/internal/jobs"

// ConflictAdvancedRule is one radio row of the "Overwrite advanced" dialog.
type ConflictAdvancedRule struct {
	Label    string
	Decision jobs.ConflictDecision
}

// conflictAdvancedRules is the single rule-to-decision table used by render and key handling.
var conflictAdvancedRules = []ConflictAdvancedRule{
	{"Overwrite if newer", jobs.DecisionOverwriteIfNewer},
	{"Overwrite if older", jobs.DecisionOverwriteIfOlder},
	{"Overwrite if existing is smaller", jobs.DecisionOverwriteIfExistingSmaller},
	{"Overwrite if size differs", jobs.DecisionOverwriteIfSizeDiffers},
	{"Overwrite if same size", jobs.DecisionOverwriteIfSameSize},
	{"Compare contents", jobs.DecisionCompare},
	{"Keep both (rename new file)", jobs.DecisionKeepBoth},
}

// ConflictAdvancedRulesFor returns the rules offered for a conflict; Compare is hidden once the
// contents are known to differ and for archive extraction (NoCompare).
func ConflictAdvancedRulesFor(c *jobs.ConflictEvent) []ConflictAdvancedRule {
	hideCompare := c != nil && (c.ContentDiffers || c.NoCompare)
	out := make([]ConflictAdvancedRule, 0, len(conflictAdvancedRules))
	for _, r := range conflictAdvancedRules {
		if hideCompare && r.Decision == jobs.DecisionCompare {
			continue
		}
		out = append(out, r)
	}
	return out
}

// ConflictAdvancedForm is the focus layout of the advanced dialog: rule radios 0..n-1, the
// apply-to-all checkbox n, then OK and Cancel. Tab jumps between those three groups.
func ConflictAdvancedForm(nRules int) DialogTrailingButtonsForm {
	return NewDialogTrailingButtonsForm(nRules+1, 2).WithSegments(0, nRules, nRules+1)
}

// ConflictAdvancedDecision returns the decision the advanced dialog would submit.
func (st ConflictDialogState) ConflictAdvancedDecision() jobs.ConflictDecision {
	rules := ConflictAdvancedRulesFor(st.Blocker.Conflict)
	if st.AdvRule < 0 || st.AdvRule >= len(rules) {
		return ""
	}
	d := rules[st.AdvRule].Decision
	if st.AdvAll {
		return d.All()
	}
	return d
}
