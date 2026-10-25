package dialog

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Button IDs of the quick job-blocker dialog. Cancel here cancels the job (ConflictCancelJob);
// the dialog's own cancel chord postpones it. Rename / Rename All replace Overwrite / Overwrite
// All for a directory mismatch.
const (
	ConflictOverwrite    = dlg.ID("overwrite")
	ConflictOverwriteAll = dlg.ID("overwrite-all")
	ConflictRename       = dlg.ID("rename")
	ConflictRenameAll    = dlg.ID("rename-all")
	ConflictAdvanced     = dlg.ID("advanced")
	ConflictSkip         = dlg.ID("skip")
	ConflictSkipAll      = dlg.ID("skip-all")
	ConflictPostpone     = dlg.ID("postpone")
	ConflictCancelJob    = dlg.ID("cancel-job")
	ConflictRetry        = dlg.ID("retry")
	ConflictAbort        = dlg.ID("abort")

	// ConflictSkipIdentical and ConflictApplyAll are the checkboxes of the Conflict rules screen.
	ConflictSkipIdentical = dlg.ID("skip-identical")
	ConflictApplyAll      = dlg.ID("apply-all")
)

// ConflictDecisionOf maps a button of the File exists / Disk space screens to the decision it
// submits. ok is false for Advanced… and Postpone, which submit nothing.
func ConflictDecisionOf(id dlg.ID) (d jobs.ConflictDecision, ok bool) {
	switch id {
	case ConflictOverwrite:
		return jobs.DecisionOverwrite, true
	case ConflictOverwriteAll:
		return jobs.DecisionOverwriteAll, true
	case ConflictRename:
		return jobs.DecisionRename, true
	case ConflictRenameAll:
		return jobs.DecisionRenameAll, true
	case ConflictSkip:
		return jobs.DecisionSkip, true
	case ConflictSkipAll:
		return jobs.DecisionSkipAll, true
	case ConflictCancelJob, ConflictAbort:
		return jobs.DecisionCancel, true
	case ConflictRetry:
		return jobs.DecisionRetry, true
	}
	return "", false
}

const (
	conflictDialogLabelNew      = "New:      "
	conflictDialogLabelExisting = "Existing: "
	conflictDialogLabelSize     = "Size:     "
	conflictDialogLabelModified = "Modified: "
)

// ConflictDialogView is the quick job-blocker modal's widget spec: File exists (with its
// directory-mismatch variant), Disk space, or, once Advanced is set, Conflict rules. Focus lives
// in st.Focus, or in st.AdvFocus on the rules screen.
func ConflictDialogView(st ConflictDialogState, userHomeDir string) dlg.Spec {
	switch {
	case st.Blocker.Kind == jobs.BlockerKindDiskSpace:
		return conflictDiskView(st, userHomeDir)
	case st.Advanced:
		return conflictRulesView(st)
	}
	return conflictFileView(st, userHomeDir)
}

// ConflictDialogFocus is the focus ID the current screen of st is driven with.
func (st *ConflictDialogState) ConflictDialogFocus() *dlg.ID {
	if st.Advanced && st.Blocker.Kind != jobs.BlockerKindDiskSpace {
		return &st.AdvFocus
	}
	return &st.Focus
}

func conflictText(label string) dlg.Item { return dlg.Item{Kind: dlg.KindLabel, Label: label} }

func conflictDisplay(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// conflictFileGroup is the New / Existing summary: path (home-tilded and fitted), size, modified.
func conflictFileGroup(pathLabel, path, size, modified, userHomeDir string) []dlg.Item {
	return []dlg.Item{
		{Kind: dlg.KindLabel, Prefix: pathLabel, Label: primitive.PathWithHomeTilde(path, userHomeDir), Fit: dlg.FitPath},
		conflictText(conflictDialogLabelSize + conflictDisplay(size)),
		conflictText(conflictDialogLabelModified + conflictDisplay(modified)),
	}
}

func conflictFileView(st ConflictDialogState, userHomeDir string) dlg.Spec {
	c := st.Blocker.Conflict
	if c == nil {
		c = &jobs.ConflictEvent{}
	}
	items := conflictFileGroup(conflictDialogLabelNew, c.Source, c.SourceSize, c.SourceTime, userHomeDir)
	items = append(items, conflictText(""))
	items = append(items, conflictFileGroup(conflictDialogLabelExisting, c.Destination, c.DestSize, c.DestTime, userHomeDir)...)
	items = append(items, dlg.Item{Kind: dlg.KindSeparator}, conflictText(ConflictPromptText(c)))

	first := []dlg.Button{
		{ID: ConflictOverwrite, Label: "Overwrite", Mnemonic: 'O', Destructive: true},
		{ID: ConflictOverwriteAll, Label: "Overwrite All", Mnemonic: 'A', Destructive: true},
		{ID: ConflictAdvanced, Label: "Advanced…", Mnemonic: 'D', Destructive: true},
	}
	if c.Mismatch {
		first = []dlg.Button{
			{ID: ConflictRename, Label: "Rename", Mnemonic: 'R'},
			{ID: ConflictRenameAll, Label: "Rename All", Mnemonic: 'A'},
		}
	}
	buttons := append(first,
		dlg.Button{ID: ConflictSkip, Label: "Skip", Mnemonic: 'S', NewRow: true},
		dlg.Button{ID: ConflictSkipAll, Label: "Skip All", Mnemonic: 'L'},
		dlg.Button{ID: ConflictPostpone, Label: "Postpone", Mnemonic: 'P'},
		dlg.Button{ID: ConflictCancelJob, Label: "Cancel", Mnemonic: 'C', NewRow: true},
	)
	return dlg.Spec{
		Title: "File exists", Width: 76, MinWidth: 48, Unclamped: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: buttons,
	}
}

func conflictDiskView(st ConflictDialogState, userHomeDir string) dlg.Spec {
	d := st.Blocker.DiskSpace
	if d == nil {
		d = &jobs.DiskSpaceBlockerDetails{}
	}
	path := func(prefix, p string) dlg.Item {
		return dlg.Item{Kind: dlg.KindLabel, Prefix: prefix, Label: primitive.PathWithHomeTilde(p, userHomeDir), Fit: dlg.FitPath}
	}
	available := "Available:   (unknown)"
	if d.AvailableKnown {
		available = "Available:   " + formatConflictDialogBytes(int64(d.AvailableBytes))
	}
	items := []dlg.Item{
		{Kind: dlg.KindLabel, Label: "Not enough free space.", Style: dlg.TextWarn},
		path("Destination: ", d.Destination),
		conflictText("Required:    " + formatConflictDialogBytes(d.RequiredBytes)),
		conflictText(available),
	}
	if d.NextSource != "" {
		items = append(items, path("Next file:   ", d.NextSource))
	}
	items = append(items,
		dlg.Item{Kind: dlg.KindSeparator},
		dlg.Item{Kind: dlg.KindLabel, Label: "Make space or retry.", Style: dlg.TextWarn})
	return dlg.Spec{
		Title: "Disk space", Width: 76, MinWidth: 48, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			{ID: ConflictRetry, Label: "Retry", Mnemonic: 'R'},
			{ID: ConflictAbort, Label: "Abort", Mnemonic: 'B', Destructive: true},
			{ID: ConflictPostpone, Label: "Postpone", Mnemonic: 'P'},
		},
	}
}

// ConflictRuleID names the radio of choice col in rule row (0..ConflictRuleRows-1).
func ConflictRuleID(row, col int) dlg.ID { return dlg.ID(fmt.Sprintf("rule-%d-%d", row, col)) }

// ConflictRuleCell is the inverse of ConflictRuleID.
func ConflictRuleCell(id dlg.ID) (row, col int, ok bool) {
	n, _ := fmt.Sscanf(string(id), "rule-%d-%d", &row, &col)
	return row, col, n == 2
}

func conflictRulesView(st ConflictDialogState) dlg.Spec {
	ruleRows := func(from, to int) []dlg.Item {
		var items []dlg.Item
		for row := from; row < to; row++ {
			cur := st.ruleAction(row)
			cells := make([]dlg.Item, len(ConflictRuleChoices))
			for col, c := range ConflictRuleChoices {
				cells[col] = dlg.Item{
					ID: ConflictRuleID(row, col), Kind: dlg.KindRadio, Label: c.Label, On: cur == c.Action,
					Disabled: !st.ConflictAdvancedChoiceEnabled(row, col),
				}
				if col < len(ConflictRuleChoices)-1 {
					cells[col].Gap = 1
				}
			}
			items = append(items, conflictText(conflictRuleRowLabels[row]),
				// A greyed choice under the cursor column falls back to Ask, which is never greyed.
				dlg.Item{Kind: dlg.KindRow, FallbackFirst: true, Cells: cells})
		}
		return items
	}
	sep := dlg.Item{Kind: dlg.KindSeparator}
	var checks []dlg.Item
	if !st.ConflictAdvancedNoCompare() {
		checks = append(checks, dlg.Item{ID: ConflictSkipIdentical, Kind: dlg.KindCheckbox, Label: "Skip identical files (slow)", Mnemonic: 'I', On: st.Rules.SkipIdentical})
	}
	checks = append(checks, dlg.Item{ID: ConflictApplyAll, Kind: dlg.KindCheckbox, Label: "Apply to all conflicts", Mnemonic: 'A', On: st.AdvAll})
	return dlg.Spec{
		Title: "Conflict rules", Width: 56, MinWidth: 52, Unclamped: true,
		Groups: []dlg.Group{
			{Columns: [][]dlg.Item{append(ruleRows(0, conflictRuleTimeRows), sep)}},
			{Columns: [][]dlg.Item{append(ruleRows(conflictRuleTimeRows, ConflictRuleRows), sep)}},
			{Columns: [][]dlg.Item{checks}},
		},
		Buttons: []dlg.Button{{ID: dlg.OK, Label: "OK", Mnemonic: 'O'}, {ID: dlg.Cancel, Label: "Cancel", Mnemonic: 'C'}},
	}
}
