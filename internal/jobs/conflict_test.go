package jobs

import (
	"testing"
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

func TestConflictPolicyConditionalRuleAll(t *testing.T) {
	p := NewConflictPolicy()
	_, _, _, updated := ApplyDecision(p, DecisionOverwriteIfNewer.All())
	if updated.Decision() != DecisionOverwriteIfNewer.All() {
		t.Fatalf("active decision = %q, want %q", updated.Decision(), DecisionOverwriteIfNewer.All())
	}
	_, _, _, single := ApplyDecision(p, DecisionOverwriteIfNewer)
	if single.Decision() != "" {
		t.Fatalf("single rule set policy %q", single.Decision())
	}
}

func TestDecisionAllBase(t *testing.T) {
	if DecisionOverwrite.All() != DecisionOverwriteAll || DecisionSkip.All() != DecisionSkipAll {
		t.Fatal("All() must match the -all constants")
	}
	if DecisionKeepBoth.All().Base() != DecisionKeepBoth {
		t.Fatal("Base() must strip the suffix")
	}
	if DecisionKeepBoth.All().All() != DecisionKeepBoth.All() {
		t.Fatal("All() must be idempotent")
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
		{DecisionCompare.All(), true},
		{DecisionKeepBoth, false},
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
