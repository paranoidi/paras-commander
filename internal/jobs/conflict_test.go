package jobs

import (
	"testing"
)

func TestConflictPolicyDefaults(t *testing.T) {
	p := NewConflictPolicy()
	if p.Decision() != "" {
		t.Fatalf("default decision = %q, want empty", p.Decision())
	}
	if p.ShouldOverwrite() {
		t.Fatal("default should not overwrite")
	}
	if p.ShouldSkip() {
		t.Fatal("default should not skip")
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
	if !updated.ShouldOverwrite() {
		t.Fatal("overwrite-all should report should overwrite")
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
