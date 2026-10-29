package dialog

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func rulesState(noCompare bool) ConflictDialogState {
	st := ConflictDialogState{
		Open:    true,
		Blocker: jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Source: "/a/harbor.txt", Destination: "/b/harbor.txt", NoCompare: noCompare}},
	}
	st.StartAdvanced()
	return st
}

func TestConflictRulesDefaults(t *testing.T) {
	st := rulesState(false)
	if !st.AdvAll || st.Rules.Verify || st.Rules.Time != [3]jobs.RuleAction{} || st.Rules.Size != [3]jobs.RuleAction{} ||
		st.Rules.Same != jobs.RuleAsk || st.Rules.Differ != jobs.RuleFallback {
		t.Fatalf("unexpected defaults: %+v", st)
	}
}

func TestConflictRulesTabJumpsBetweenGroups(t *testing.T) {
	st := rulesState(false)
	spec := st.Screen("").Spec
	for from, want := range map[dlg.ID]dlg.ID{
		ConflictRuleID(0, 0): ConflictRuleID(3, 0),
		ConflictRuleID(2, 2): ConflictRuleID(3, 2),
		ConflictRuleID(3, 0): ConflictVerify,
		ConflictRuleID(5, 0): ConflictVerify,
		ConflictVerify:       ConflictApplyAll,
		ConflictApplyAll:     dlg.OK,
		dlg.Cancel:           ConflictRuleID(0, 0),
	} {
		focus := from
		dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
		if want == ConflictRuleID(3, 2) {
			want = ConflictRuleID(3, 0) // a segment starts at its first item
		}
		if focus != want {
			t.Fatalf("Tab from %q = %q, want %q", from, focus, want)
		}
	}
	// Without Verify the next group is just Apply to all.
	ns := rulesState(true)
	noCompare := ns.Screen("").Spec
	focus := ConflictRuleID(5, 0)
	dlg.HandleKey(noCompare, &focus, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if focus != ConflictApplyAll {
		t.Fatalf("Tab into the checkboxes without Verify = %q, want Apply to all", focus)
	}
}

func TestConflictRulesApplyRuleSkipsGreyed(t *testing.T) {
	st := rulesState(false)
	st.ApplyRule(ConflictRuleID(0, 2)) // Destination newer: Skip
	st.ApplyRule(ConflictRuleID(1, 1)) // Destination older: Overwrite

	// Skip and Overwrite now contradict: only Ask is enabled in the size rows, and the view
	// greys the rest.
	spec := st.Screen("").Spec
	size := spec.Groups[1].Columns[0][0] // the "Destination smaller" row: label, then radios
	for col, c := range size.Cells[1:] {
		if want := col != 0; c.Disabled != want {
			t.Fatalf("size choice %d disabled = %v, want %v", col, c.Disabled, want)
		}
	}
	st.ApplyRule(ConflictRuleID(5, 1))
	if st.Rules.Size[jobs.SizeSame] != jobs.RuleAsk {
		t.Fatal("greyed choice must not be selectable")
	}

	// Dropping the time conflict re-enables a matching size choice.
	st.ApplyRule(ConflictRuleID(1, 2)) // older: Skip, same as newer
	st.ApplyRule(ConflictRuleID(5, 2))
	if st.Rules.Size[jobs.SizeSame] != jobs.RuleSkip {
		t.Fatal("Skip must be selectable when the time rows only hold Skip")
	}
	// A time change that contradicts it resets the size row to Ask.
	st.ApplyRule(ConflictRuleID(0, 1))
	if st.Rules.Size[jobs.SizeSame] != jobs.RuleAsk {
		t.Fatalf("size selection must fall back to Ask, got %v", st.Rules.Size[jobs.SizeSame])
	}
	// Ids that are not a rule radio change nothing.
	before := st.Rules
	st.ApplyRule(ConflictApplyAll)
	st.ApplyRule("rule-9-9")
	if st.Rules != before {
		t.Fatal("a non-rule id changed the rules")
	}
}

func TestConflictRulesChoiceNavigationSkipsGreyed(t *testing.T) {
	st := rulesState(false)
	st.ApplyRule(ConflictRuleID(0, 2)) // time rows hold Skip: Overwrite and Keep both are greyed below
	spec := st.Screen("").Spec
	focus := ConflictRuleID(3, 0)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyRight, 0, tcell.ModNone))
	if focus != ConflictRuleID(3, 2) {
		t.Fatalf("Right from Ask = %q, want the Skip choice (greyed ones are skipped)", focus)
	}
	// Down onto a greyed column lands on the nearest choice that is not greyed.
	focus = ConflictRuleID(2, 3)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if focus != ConflictRuleID(3, 2) {
		t.Fatalf("Down onto a greyed column = %q, want the nearest choice Skip", focus)
	}
}

func TestConflictRulesAnswer(t *testing.T) {
	st := rulesState(false)
	st.Rules.Verify = true
	a := st.ConflictAdvancedAnswer()
	if a.Decision != jobs.DecisionRules.All() || a.Rules == nil || !a.Rules.Verify {
		t.Fatalf("answer = %+v", a)
	}
	st.AdvAll = false
	if got := st.ConflictAdvancedAnswer().Decision; got != jobs.DecisionRules {
		t.Fatalf("decision = %q", got)
	}
	ext := rulesState(true)
	ext.Rules.Verify = true
	if ext.ConflictAdvancedAnswer().Rules.Verify {
		t.Fatal("extract must not verify")
	}
}

func moveRulesState() ConflictDialogState {
	st := rulesState(false)
	st.Blocker.Conflict.Move = true
	return st
}

func TestConflictRulesVerifyGreying(t *testing.T) {
	st := rulesState(false)
	// Verify off: Same and Differ are greyed, the matrix is live.
	for _, row := range []int{ConflictRuleSameRow, ConflictRuleDifferRow} {
		if st.ConflictAdvancedChoiceEnabled(row, 0) {
			t.Fatalf("row %d must be greyed without Verify", row)
		}
	}
	st.ApplyRule(ConflictRuleID(ConflictRuleSameRow, 2))
	if st.Rules.Same != jobs.RuleAsk {
		t.Fatal("a greyed Same choice must not be selectable")
	}
	st.ToggleVerify()
	// Verify on with Differ = Rules: everything is live.
	if !st.ConflictAdvancedChoiceEnabled(0, 1) || !st.ConflictAdvancedChoiceEnabled(ConflictRuleSameRow, 2) {
		t.Fatal("Verify with the Differ fallback keeps the matrix and Same live")
	}
	// Differ decides every differing file: the time and size rows go grey.
	st.ApplyRule(ConflictRuleID(ConflictRuleDifferRow, 4)) // Keep both
	if st.Rules.Differ != jobs.RuleKeepBoth {
		t.Fatalf("Differ = %v, want Keep both", st.Rules.Differ)
	}
	for row := range 6 {
		if st.ConflictAdvancedChoiceEnabled(row, 0) {
			t.Fatalf("row %d must be greyed when Differ decides", row)
		}
	}
	st.ApplyRule(ConflictRuleID(0, 1))
	if st.Rules.Time[jobs.TimeDestNewer] != jobs.RuleAsk {
		t.Fatal("a greyed time choice must not be selectable")
	}
	// Turning Verify off keeps the choices but brings the matrix back.
	st.ToggleVerify()
	if st.Rules.Differ != jobs.RuleKeepBoth || !st.ConflictAdvancedChoiceEnabled(0, 1) {
		t.Fatal("Verify off keeps Differ and re-enables the matrix")
	}
}

func TestConflictRulesDeleteSourceOnlyForMove(t *testing.T) {
	cp := rulesState(false)
	if n := len(cp.ConflictRuleChoices(ConflictRuleSameRow)); n != 3 {
		t.Fatalf("copy Same choices = %d, want 3", n)
	}
	if n := len(cp.ConflictRuleChoices(ConflictRuleDifferRow)); n != 5 {
		t.Fatalf("copy Differ choices = %d, want 5", n)
	}
	cp.ToggleVerify()
	cp.ApplyRule(ConflictRuleID(ConflictRuleSameRow, 3))
	if cp.Rules.Same != jobs.RuleAsk {
		t.Fatal("a copy cannot pick Delete source")
	}
	mv := moveRulesState()
	mv.ToggleVerify()
	mv.ApplyRule(ConflictRuleID(ConflictRuleSameRow, 3))
	mv.ApplyRule(ConflictRuleID(ConflictRuleDifferRow, 5))
	if mv.Rules.Same != jobs.RuleDeleteSource || mv.Rules.Differ != jobs.RuleDeleteSource {
		t.Fatalf("move rules = %+v, want Delete source on both", mv.Rules)
	}
	if a := mv.ConflictAdvancedAnswer(); a.Rules.Same != jobs.RuleDeleteSource {
		t.Fatal("the answer keeps Delete source for a move")
	}
}

func TestConflictRulesDifferWrapNavigation(t *testing.T) {
	st := moveRulesState()
	st.ToggleVerify()
	spec := st.Screen("").Spec
	// Down from Differ's Overwrite (first line) lands on its second line.
	focus := ConflictRuleID(ConflictRuleDifferRow, 2)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if focus != ConflictRuleID(ConflictRuleDifferRow, 4) {
		t.Fatalf("Down from the first Differ line = %q, want the second line", focus)
	}
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if focus != ConflictApplyAll {
		t.Fatalf("Down from the second Differ line = %q, want Apply to all", focus)
	}
}

func TestConflictRulesDialogLayout(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	st := rulesState(false)
	DrawConflictDialog(screen, layout, st, theme.Default(), "")
	rows := dialogScreenRows(screen)
	all := strings.Join(rows, "\n")
	for _, want := range []string{"Conflict rules", "Destination newer", "Same size", "Keep both", "Compare files", "Differ", "Apply to all conflicts"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in:\n%s", want, all)
		}
	}
	if n := strings.Count(all, "Ask"); n != 8 {
		t.Fatalf("Ask choices = %d, want 8", n)
	}
	if strings.Contains(all, "Delete source") {
		t.Fatal("a copy offers no Delete source")
	}
	// 18 rows incl. borders, so the dialog fits a 24-row terminal.
	height := 0
	for _, r := range rows {
		if strings.ContainsAny(r, "│┌└├") {
			height++
		}
	}
	if height != 18 {
		t.Fatalf("dialog height = %d, want 18:\n%s", height, all)
	}
	assertLabelsAreFollowedByContent(t, rows)

	st = rulesState(true)
	screen = buttonBlankRowScreen(t)
	DrawConflictDialog(screen, layout, st, theme.Default(), "")
	if strings.Contains(strings.Join(dialogScreenRows(screen), "\n"), "Compare files") {
		t.Fatal("Compare files must be hidden for extract")
	}
}

func assertLabelsAreFollowedByContent(t *testing.T, rows []string) {
	t.Helper()
	for _, r := range rows {
		if strings.Contains(r, "Destination newer") && !strings.Contains(r, "Overwrite") {
			t.Fatalf("label row has no choices: %q", r)
		}
	}
}
