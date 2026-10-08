package jobs

import (
	"testing"
	"time"
)

func TestConflictPolicyDefaults(t *testing.T) {
	p := NewConflictPolicy()
	if p.Decision() != "" {
		t.Fatalf("default decision = %q, want empty", p.Decision())
	}
}

func TestConflictPolicyOverwrite(t *testing.T) {
	p := NewConflictPolicy()
	overwrite, skip, cancel, updated := ApplyDecision(p, DecisionOverwrite)
	if !overwrite {
		t.Fatal("expected overwrite")
	}
	if skip {
		t.Fatal("expected no skip")
	}
	if cancel {
		t.Fatal("expected no cancel")
	}
	if updated.Decision() != "" {
		t.Fatalf("overwrite should not set active decision, got %q", updated.Decision())
	}
}

func TestConflictPolicyOverwriteAll(t *testing.T) {
	p := NewConflictPolicy()
	overwrite, skip, cancel, updated := ApplyDecision(p, DecisionOverwriteAll)
	if !overwrite {
		t.Fatal("expected overwrite")
	}
	if skip {
		t.Fatal("expected no skip")
	}
	if cancel {
		t.Fatal("expected no cancel")
	}
	if updated.Decision() != DecisionOverwriteAll {
		t.Fatalf("expected active decision overwrite-all, got %q", updated.Decision())
	}
}

func TestConflictPolicySkipAll(t *testing.T) {
	p := NewConflictPolicy()
	overwrite, skip, cancel, updated := ApplyDecision(p, DecisionSkipAll)
	if overwrite {
		t.Fatal("expected no overwrite")
	}
	if !skip {
		t.Fatal("expected skip")
	}
	if cancel {
		t.Fatal("expected no cancel")
	}
	if updated.Decision() != DecisionSkipAll {
		t.Fatalf("expected active decision skip-all, got %q", updated.Decision())
	}
}

func TestConflictPolicyRulesAll(t *testing.T) {
	p := NewConflictPolicy()
	_, _, _, updated := ApplyDecision(p, DecisionRules.All())
	if updated.Decision() != DecisionRules.All() {
		t.Fatalf("active decision = %q, want %q", updated.Decision(), DecisionRules.All())
	}
	_, _, _, single := ApplyDecision(p, DecisionRules)
	if single.Decision() != "" {
		t.Fatalf("single rules answer set policy %q", single.Decision())
	}
}

func TestDecisionAllBase(t *testing.T) {
	if DecisionOverwrite.All() != DecisionOverwriteAll || DecisionSkip.All() != DecisionSkipAll {
		t.Fatal("All() must match the -all constants")
	}
	if DecisionRules.All().Base() != DecisionRules {
		t.Fatal("Base() must strip the suffix")
	}
	if DecisionRules.All().All() != DecisionRules.All() {
		t.Fatal("All() must be idempotent")
	}
}

func TestConflictRulesEvaluate(t *testing.T) {
	base := time.Unix(1_000_000, 0)
	later := base.Add(time.Hour)
	tests := []struct {
		name         string
		rules        ConflictRules
		src, dst     time.Time
		srcSz, dstSz int64
		want         RuleAction
	}{
		{"all ask", ConflictRules{}, base, later, 1, 2, RuleAsk},
		{"time row wins", ConflictRules{Time: [3]RuleAction{TimeDestNewer: RuleSkip}, Size: [3]RuleAction{SizeDestLarger: RuleOverwrite}}, base, later, 1, 2, RuleSkip},
		{"size row when time asks", ConflictRules{Size: [3]RuleAction{SizeDestLarger: RuleKeepBoth}}, base, later, 1, 2, RuleKeepBoth},
		{"older row", ConflictRules{Time: [3]RuleAction{TimeDestOlder: RuleOverwrite}}, later, base, 1, 1, RuleOverwrite},
		{"same time ignores sub-second", ConflictRules{Time: [3]RuleAction{TimeSame: RuleSkip}}, base, base.Add(400 * time.Millisecond), 1, 1, RuleSkip},
		{"smaller row", ConflictRules{Size: [3]RuleAction{SizeDestSmaller: RuleOverwrite}}, base, base, 5, 2, RuleOverwrite},
		{"same size row", ConflictRules{Size: [3]RuleAction{SizeSame: RuleSkip}}, base, base, 2, 2, RuleSkip},
	}
	for _, tt := range tests {
		if got := tt.rules.Evaluate(tt.src, tt.dst, tt.srcSz, tt.dstSz); got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestConflictRulesGreyingAndNormalize(t *testing.T) {
	r := ConflictRules{Time: [3]RuleAction{TimeDestNewer: RuleSkip}}
	if !r.SizeChoiceEnabled(RuleSkip) || !r.SizeChoiceEnabled(RuleAsk) || r.SizeChoiceEnabled(RuleOverwrite) {
		t.Fatal("only Skip and Ask may stay enabled when a time row is Skip")
	}
	r.Size[SizeSame] = RuleSkip
	r.Time[TimeDestOlder] = RuleOverwrite // now Skip and Overwrite conflict: only Ask is enabled
	r.Normalize()
	if r.Size[SizeSame] != RuleAsk {
		t.Fatalf("greyed selection must fall back to Ask, got %v", r.Size[SizeSame])
	}
}

func TestConflictPolicyCancel(t *testing.T) {
	p := NewConflictPolicy()
	overwrite, skip, cancel, updated := ApplyDecision(p, DecisionCancel)
	if overwrite {
		t.Fatal("expected no overwrite")
	}
	if skip {
		t.Fatal("expected no skip")
	}
	if !cancel {
		t.Fatal("expected cancel")
	}
	if updated.Decision() != "" {
		t.Fatalf("cancel should not set active decision, got %q", updated.Decision())
	}
}

func TestConflictPolicyUsesActiveDecision(t *testing.T) {
	p := NewConflictPolicy()
	p.SetDecision(DecisionOverwriteAll)

	// When decision is empty, use active policy.
	overwrite, skip, cancel, _ := ApplyDecision(p, "")
	if !overwrite {
		t.Fatal("expected overwrite from active policy")
	}
	if skip {
		t.Fatal("expected no skip")
	}
	if cancel {
		t.Fatal("expected no cancel")
	}
}

func TestApplyAll(t *testing.T) {
	tests := []struct {
		decision ConflictDecision
		applyAll bool
	}{
		{DecisionOverwriteAll, true},
		{DecisionSkipAll, true},
		{DecisionRules.All(), true},
		{DecisionRules, false},
		{DecisionOverwrite, false},
		{DecisionSkip, false},
		{DecisionCancel, false},
	}
	for _, tt := range tests {
		if tt.decision.ApplyAll() != tt.applyAll {
			t.Fatalf("%q ApplyAll() = %v, want %v", tt.decision, tt.decision.ApplyAll(), tt.applyAll)
		}
	}
}
