package dialog

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/theme"
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

func TestConflictRulesFormTabJumpsBetweenGroups(t *testing.T) {
	st := rulesState(false)
	f := st.ConflictAdvancedForm()
	for from, want := range map[int]int{0: 3, 2: 3, 3: 6, 5: 6, 6: f.OKIndex(), 7: f.OKIndex(), f.CancelIndex(): 0} {
		if got := f.Tab(from); got != want {
			t.Fatalf("Tab(%d) = %d, want %d", from, got, want)
		}
	}
	if f2 := rulesState(true).ConflictAdvancedForm(); f2.OKIndex() != 7 {
		t.Fatalf("OK index without Skip identical = %d, want 7", f2.OKIndex())
	}
}

func TestConflictRulesChoiceNavigationSkipsGreyed(t *testing.T) {
	st := rulesState(false)
	st.AdvFocus = 0 // Destination newer: pick Skip
	st.AdvCol = 2
	st.AdvSelectChoice()
	st.AdvFocus = 1
	st.SyncAdvCol()
	st.AdvCol = 1 // Destination older: Overwrite
	st.AdvSelectChoice()

	// Skip and Overwrite now contradict: only Ask is enabled in the size rows.
	st.AdvFocus = ConflictRuleRows - 1
	st.SyncAdvCol()
	if st.AdvCol != 0 {
		t.Fatalf("cursor = %d, want Ask", st.AdvCol)
	}
	st.AdvMoveChoice(1)
	if st.AdvCol != 0 {
		t.Fatalf("Right must skip greyed choices, cursor = %d", st.AdvCol)
	}
	st.AdvCol = 1
	st.AdvSelectChoice()
	if st.Rules.Size[jobs.SizeSame] != jobs.RuleAsk {
		t.Fatal("greyed choice must not be selectable")
	}

	// Dropping the time conflict re-enables a matching size choice.
	st.AdvFocus = 1
	st.AdvCol = 2
	st.AdvSelectChoice() // older: Skip, same as newer
	st.AdvFocus = 5
	st.AdvCol = 2
	st.AdvSelectChoice()
	if st.Rules.Size[jobs.SizeSame] != jobs.RuleSkip {
		t.Fatal("Skip must be selectable when the time rows only hold Skip")
	}
	// A time change that contradicts it resets the size row to Ask.
	st.AdvFocus = 0
	st.AdvCol = 1
	st.AdvSelectChoice()
	if st.Rules.Size[jobs.SizeSame] != jobs.RuleAsk {
		t.Fatalf("size selection must fall back to Ask, got %v", st.Rules.Size[jobs.SizeSame])
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
	assertSurfaceBlankRowAboveButtons(t, rows)
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

func TestConflictRulesRowChangeKeepsColumn(t *testing.T) {
	st := rulesState(false)
	st.AdvCol = 1 // Overwrite on "Destination newer"
	st.AdvFocus = 1
	st.SyncAdvCol()
	if st.AdvCol != 1 {
		t.Fatalf("cursor = %d, want column kept at Overwrite", st.AdvCol)
	}
}
