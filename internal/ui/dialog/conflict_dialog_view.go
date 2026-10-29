package dialog

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
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
	ConflictDeleteSource = dlg.ID("delete-source")
	ConflictSkip         = dlg.ID("skip")
	ConflictSkipAll      = dlg.ID("skip-all")
	ConflictPostpone     = dlg.ID("postpone")
	ConflictCancelJob    = dlg.ID("cancel-job")
	ConflictRetry        = dlg.ID("retry")
	ConflictAbort        = dlg.ID("abort")

	// ConflictVerify and ConflictApplyAll are the checkboxes of the Conflict rules screen.
	ConflictVerify   = dlg.ID("verify")
	ConflictApplyAll = dlg.ID("apply-all")
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
	case ConflictDeleteSource:
		return jobs.DecisionDeleteSource, true
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

// Screen is the quick job-blocker modal's screen on top: File exists (with its directory-mismatch
// variant) or Disk space, driven with st.Focus, or, once Advanced is set on a File exists blocker,
// the Conflict rules driven with st.AdvFocus.
func (st *ConflictDialogState) Screen(userHomeDir string) DialogScreen {
	switch {
	case st.Blocker.Kind == jobs.BlockerKindDiskSpace:
		return DialogScreen{conflictDiskView(*st, userHomeDir), &st.Focus}
	case st.Advanced:
		return DialogScreen{conflictRulesView(*st), &st.AdvFocus}
	}
	return DialogScreen{conflictFileView(*st, userHomeDir), &st.Focus}
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
	return dlg.Spec{
		Title: "File exists", Width: 76, MinWidth: 48, Unclamped: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: ConflictFileButtons(c),
	}
}

// ConflictFileButtons are the File exists buttons in focus order, rows started by NewRow:
// Overwrite / Overwrite All / Advanced… (Rename / Rename All for a directory mismatch), Skip /
// Skip All / Postpone, then Delete source (a move without a mismatch) and Cancel. The jobs-view
// blocker panel shows the same buttons minus Postpone.
func ConflictFileButtons(c *jobs.ConflictEvent) []dlg.Button {
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
		dlg.Button{ID: ConflictSkipAll, Label: "Skip All", Mnemonic: 'K'},
		dlg.Button{ID: ConflictPostpone, Label: "Postpone", Mnemonic: 'P'},
	)
	cancel := dlg.Button{ID: ConflictCancelJob, Label: "Cancel", Mnemonic: 'C', NewRow: true}
	if c.Move && !c.Mismatch {
		buttons = append(buttons, dlg.Button{ID: ConflictDeleteSource, Label: "Delete source", Mnemonic: 'E', Destructive: true, NewRow: true})
		cancel.NewRow = false
	}
	return append(buttons, cancel)
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

// conflictRuleLabelWidth is the label column of the rule rows: the longest label plus a gap.
const conflictRuleLabelWidth = len("Destination smaller") + 1

// Differ choice columns per line: Rules sits in the label column so Ask, Overwrite, Skip and
// Delete source line up with the Same row; Keep both wraps under Ask.
var (
	conflictDifferFirstLine  = []int{0, 1, 2, 3, 5}
	conflictDifferSecondLine = []int{4}
)

// conflictRuleGroups are the rule groups of the Conflict rules screen: time, size and the Verify rows.
func conflictRuleGroups(st ConflictDialogState) []dlg.Group {
	// ruleRow is one line of rule row: its label in a labelW box (blank on a continuation line)
	// and the radios of choices cols (columns past the row's choices are left out).
	ruleRow := func(row int, cols []int, label string, labelW int) dlg.Item {
		choices := st.ConflictRuleChoices(row)
		cur := st.ruleAction(row)
		cells := []dlg.Item{{Kind: dlg.KindLabel, Label: label, Width: labelW}}
		for _, col := range cols {
			if col >= len(choices) {
				continue
			}
			c := choices[col]
			cells = append(cells, dlg.Item{
				ID: ConflictRuleID(row, col), Kind: dlg.KindRadio, Label: c.Label, On: cur == c.Action,
				Disabled: !st.ConflictAdvancedChoiceEnabled(row, col), Gap: 1,
			})
		}
		cells[len(cells)-1].Gap = 0
		return dlg.Item{Kind: dlg.KindRow, Cells: cells}
	}
	allCols := func(n int) []int {
		cols := make([]int, n)
		for i := range cols {
			cols[i] = i
		}
		return cols
	}
	ruleRows := func(from, to int) []dlg.Item {
		var items []dlg.Item
		for row := from; row < to; row++ {
			items = append(items, ruleRow(row, allCols(len(conflictMatrixChoices)), conflictRuleRowLabels[row], conflictRuleLabelWidth))
		}
		return items
	}
	// The Differ label shares its column with the Rules radio and its gap.
	differLabelW := conflictRuleLabelWidth - draw.DialogOptionWidth(conflictDifferChoices[0].Label) - 1
	sep := dlg.Item{Kind: dlg.KindSeparator}
	groups := []dlg.Group{
		{Columns: [][]dlg.Item{append(ruleRows(0, conflictRuleTimeRows), sep)}},
		{Columns: [][]dlg.Item{append(ruleRows(conflictRuleTimeRows, conflictRuleSizeEnd), sep)}},
	}
	if !st.ConflictAdvancedNoCompare() {
		groups = append(groups, dlg.Group{Columns: [][]dlg.Item{{
			{ID: ConflictVerify, Kind: dlg.KindCheckbox, Label: "Compare files", Mnemonic: 'F', On: st.Rules.Verify},
			ruleRow(ConflictRuleSameRow, allCols(len(conflictSameChoices)), conflictRuleRowLabels[ConflictRuleSameRow], conflictRuleLabelWidth),
			ruleRow(ConflictRuleDifferRow, conflictDifferFirstLine, conflictRuleRowLabels[ConflictRuleDifferRow], differLabelW),
			ruleRow(ConflictRuleDifferRow, conflictDifferSecondLine, "", conflictRuleLabelWidth),
			sep,
		}}})
	}
	return groups
}

func conflictRulesView(st ConflictDialogState) dlg.Spec {
	groups := conflictRuleGroups(st)
	groups = append(groups, dlg.Group{Columns: [][]dlg.Item{{
		{ID: ConflictApplyAll, Kind: dlg.KindCheckbox, Label: "Apply to all conflicts", Mnemonic: 'A', On: st.AdvAll},
	}}})
	return dlg.Spec{
		Title: "Conflict rules", Width: 76, MinWidth: 72, Unclamped: true,
		Groups:  groups,
		Buttons: dlg.OKCancelButtons(),
	}
}
