package dialog

import (
	"strings"
	"testing"

	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/theme"
)

func TestConflictAdvancedRulesHideCompare(t *testing.T) {
	has := func(rules []ConflictAdvancedRule) bool {
		for _, r := range rules {
			if r.Decision == jobs.DecisionCompare {
				return true
			}
		}
		return false
	}
	if !has(ConflictAdvancedRulesFor(&jobs.ConflictEvent{})) || !has(ConflictAdvancedRulesFor(nil)) {
		t.Fatal("Compare must be offered while contents are unknown")
	}
	if has(ConflictAdvancedRulesFor(&jobs.ConflictEvent{ContentDiffers: true})) {
		t.Fatal("Compare must be hidden once contents differ")
	}
	if has(ConflictAdvancedRulesFor(&jobs.ConflictEvent{NoCompare: true})) {
		t.Fatal("Compare must be hidden for archive extraction")
	}
}

func TestConflictAdvancedFormTabJumpsBetweenGroups(t *testing.T) {
	n := len(ConflictAdvancedRulesFor(nil))
	f := ConflictAdvancedForm(n)
	if got := f.Tab(2); got != n {
		t.Fatalf("Tab from radio = %d, want checkbox %d", got, n)
	}
	if got := f.Tab(n); got != f.OKIndex() {
		t.Fatalf("Tab from checkbox = %d, want OK %d", got, f.OKIndex())
	}
	if got := f.Tab(f.CancelIndex()); got != 0 {
		t.Fatalf("Tab from buttons = %d, want wrap to 0", got)
	}
}

func TestConflictAdvancedDecision(t *testing.T) {
	st := ConflictDialogState{Blocker: jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{}}, AdvRule: 1}
	if got := st.ConflictAdvancedDecision(); got != jobs.DecisionOverwriteIfOlder {
		t.Fatalf("decision = %q", got)
	}
	st.AdvAll = true
	if got := st.ConflictAdvancedDecision(); got != jobs.DecisionOverwriteIfOlder.All() {
		t.Fatalf("decision = %q", got)
	}
}

func TestConflictAdvancedDialogLayout(t *testing.T) {
	screen := buttonBlankRowScreen(t)
	layout := Layout{Width: buttonBlankRowTermW, Height: buttonBlankRowTermH}
	DrawConflictDialog(screen, layout, ConflictDialogState{
		Open:     true,
		Advanced: true,
		Blocker:  jobs.BlockerDetails{Kind: jobs.BlockerKindConflict, Conflict: &jobs.ConflictEvent{Source: "/a/harbor.txt", Destination: "/b/harbor.txt"}},
	}, theme.Default(), "")
	rows := dialogScreenRows(screen)
	assertSurfaceBlankRowAboveButtons(t, rows)
	all := strings.Join(rows, "\n")
	for _, want := range []string{"Overwrite advanced", "Overwrite if newer", "Compare contents", "Keep both (rename new file)", "Apply to all conflicts"} {
		if !strings.Contains(all, want) {
			t.Fatalf("missing %q in:\n%s", want, all)
		}
	}
}
