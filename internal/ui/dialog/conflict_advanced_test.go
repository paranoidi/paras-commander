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
	if !st.AdvAll || st.Rules.SkipIdentical || st.Rules.Time != [3]jobs.RuleAction{} || st.Rules.Size != [3]jobs.RuleAction{} {
		t.Fatalf("unexpected defaults: %+v", st)
	}
}

func TestConflictRulesTabJumpsBetweenGroups(t *testing.T) {
	st := rulesState(false)
	spec := st.Screen("").Spec
	for from, want := range map[dlg.ID]dlg.ID{
		ConflictRuleID(0, 0):  ConflictRuleID(3, 0),
		ConflictRuleID(2, 2):  ConflictRuleID(3, 2),
		ConflictRuleID(3, 0):  ConflictSkipIdentical,
		ConflictRuleID(5, 0):  ConflictSkipIdentical,
		ConflictSkipIdentical: dlg.OK,
		ConflictApplyAll:      dlg.OK,
		dlg.Cancel:            ConflictRuleID(0, 0),
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
	// Without Skip identical the checkbox group is just Apply to all.
	ns := rulesState(true)
	noCompare := ns.Screen("").Spec
	focus := ConflictRuleID(5, 0)
	dlg.HandleKey(noCompare, &focus, tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone))
	if focus != ConflictApplyAll {
		t.Fatalf("Tab into the checkboxes without Skip identical = %q, want Apply to all", focus)
	}
}

func TestConflictRulesApplyRuleSkipsGreyed(t *testing.T) {
	st := rulesState(false)
	st.ApplyRule(ConflictRuleID(0, 2)) // Destination newer: Skip
	st.ApplyRule(ConflictRuleID(1, 1)) // Destination older: Overwrite

	// Skip and Overwrite now contradict: only Ask is enabled in the size rows, and the view
	// greys the rest.
	spec := st.Screen("").Spec
	size := spec.Groups[1].Columns[0][1] // the "Destination smaller" row
	for col, c := range size.Cells {
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
	// Down from a greyed column falls back to Ask, not to the nearest choice.
	focus = ConflictRuleID(2, 3)
	dlg.HandleKey(spec, &focus, tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone))
	if focus != ConflictRuleID(3, 0) {
		t.Fatalf("Down onto a greyed column = %q, want Ask", focus)
	}
}

func TestConflictRulesAnswer(t *testing.T) {
	st := rulesState(false)
	st.Rules.SkipIdentical = true
	a := st.ConflictAdvancedAnswer()
	if a.Decision != jobs.DecisionRules.All() || a.Rules == nil || !a.Rules.SkipIdentical {
		t.Fatalf("answer = %+v", a)
	}
	st.AdvAll = false
	if got := st.ConflictAdvancedAnswer().Decision; got != jobs.DecisionRules {
		t.Fatalf("decision = %q", got)
	}
	ext := rulesState(true)
	ext.Rules.SkipIdentical = true
	if ext.ConflictAdvancedAnswer().Rules.SkipIdentical {
		t.Fatal("extract must not skip identical")
	}
}

func TestConflictRulesDialogLayout(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	st := rulesState(false)
	DrawConflictDialog(screen, layout, st, theme.Default(), "")
	rows := dialogScreenRows(screen)
	all := strings.Join(rows, "\n")
	for _, want := range []string{"Conflict rules", "Destination newer", "Same size", "Keep both", "Skip identical files", "Apply to all conflicts"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in:\n%s", want, all)
		}
	}
	if n := strings.Count(all, "Ask"); n != 6 {
		t.Fatalf("Ask choices = %d, want 6", n)
	}
	// 20 rows incl. borders, so the dialog fits a 24-row terminal.
	height := 0
	for _, r := range rows {
		if strings.ContainsAny(r, "│┌└├") {
			height++
		}
	}
	if height != 20 {
		t.Fatalf("dialog height = %d, want 20:\n%s", height, all)
	}
	assertLabelsAreFollowedByContent(t, rows)

	st = rulesState(true)
	screen = buttonBlankRowScreen(t)
	DrawConflictDialog(screen, layout, st, theme.Default(), "")
	if strings.Contains(strings.Join(dialogScreenRows(screen), "\n"), "Skip identical files") {
		t.Fatal("Skip identical must be hidden for extract")
	}
}

func assertLabelsAreFollowedByContent(t *testing.T, rows []string) {
	t.Helper()
	for i, r := range rows {
		if strings.Contains(r, "Destination newer") && !strings.Contains(rows[i+1], "Overwrite") {
			t.Fatalf("row under label is %q", rows[i+1])
		}
	}
}
