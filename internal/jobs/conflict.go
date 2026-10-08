package jobs

import (
	"strings"
	"time"
)

// ConflictDecision is the user's choice for resolving a file conflict.
type ConflictDecision string

const (
	DecisionOverwrite ConflictDecision = "overwrite"
	DecisionSkip      ConflictDecision = "skip"
	DecisionCancel    ConflictDecision = "cancel"
	// DecisionRetry applies to disk-space blockers only (re-check free space and continue).
	DecisionRetry ConflictDecision = "retry"

	// DecisionRules means "evaluate the attached ConflictRules per file" (see BlockerAnswer.Rules).
	DecisionRules ConflictDecision = "rules"

	DecisionOverwriteAll = DecisionOverwrite + allSuffix
	DecisionSkipAll      = DecisionSkip + allSuffix
)

const allSuffix = "-all"

// All returns the apply-to-all form of d.
func (d ConflictDecision) All() ConflictDecision {
	if d.ApplyAll() {
		return d
	}
	return d + allSuffix
}

// Base returns d without the apply-to-all suffix.
func (d ConflictDecision) Base() ConflictDecision {
	return ConflictDecision(strings.TrimSuffix(string(d), allSuffix))
}

// ApplyAll reports whether the decision applies to all remaining conflicts
// in the current job.
func (d ConflictDecision) ApplyAll() bool {
	return strings.HasSuffix(string(d), allSuffix)
}

// ConflictRequest represents a user-facing conflict that requires a decision.
type ConflictRequest struct {
	JobID           string
	Source          string
	Destination     string
	ExistingDetails string
	SourceSize      string
	SourceTime      string
	DestSize        string
	DestTime        string
	// Reprompt is the note for a repeat prompt after the rules left this file undecided
	// ("No rule matched."); a non-empty value bypasses the job's apply-to-all policy.
	Reprompt string
	// NoCompare hides Skip identical: the source is not comparable to the destination (archive extract).
	NoCompare bool
}

// ConflictPolicy tracks active overwrite/skip decisions within a job.
type ConflictPolicy struct {
	activeDecision ConflictDecision
	rules          *ConflictRules
}

// NewConflictPolicy creates a clean conflict policy with no active bulk decision.
func NewConflictPolicy() ConflictPolicy {
	return ConflictPolicy{}
}

// Decision returns the current active decision or empty string if none.
func (p ConflictPolicy) Decision() ConflictDecision {
	return p.activeDecision
}

// Answer returns the active decision together with its rules.
func (p ConflictPolicy) Answer() BlockerAnswer {
	return BlockerAnswer{Decision: p.activeDecision, Rules: p.rules}
}

// SetDecision sets a new active decision.
func (p *ConflictPolicy) SetDecision(d ConflictDecision) {
	p.activeDecision = d
}

// ApplyDecision determines the effective conflict outcome for a single file
// given the current policy and a new decision (which may be empty to reuse policy).
// It returns (shouldOverwrite, shouldSkip, shouldCancel, updatedPolicy).
func ApplyDecision(policy ConflictPolicy, newDecision ConflictDecision) (overwrite, skip, cancel bool, updated ConflictPolicy) {
	decision := newDecision
	if decision == "" {
		decision = policy.activeDecision
	}

	if decision == DecisionRetry {
		// Not a conflict outcome; must not be routed through ApplyDecision from conflict UI.
		return false, false, false, policy
	}
	if decision.ApplyAll() {
		policy.activeDecision = decision
	}
	switch decision.Base() {
	case DecisionOverwrite:
		return true, false, false, policy
	case DecisionSkip:
		return false, true, false, policy
	case DecisionCancel:
		return false, false, true, policy
	default:
		return false, false, false, policy
	}
}

// BlockerAnswer is the user's reply to a blocker: a decision plus, for DecisionRules, the rules.
type BlockerAnswer struct {
	Decision ConflictDecision
	Rules    *ConflictRules
}

// RuleAction is what a conflict-rules row does with a matching file.
type RuleAction int

const (
	RuleAsk RuleAction = iota // no rule: leave the decision to the next row, else prompt again
	RuleOverwrite
	RuleSkip
	RuleKeepBoth // write the new file under the first free "name (N).ext"
)

// Row indexes of ConflictRules.Time and ConflictRules.Size (relative to the destination).
const (
	TimeDestNewer = iota
	TimeDestOlder
	TimeSame
)
const (
	SizeDestSmaller = iota
	SizeDestLarger
	SizeSame
)

// ConflictRules is the "Conflict rules" matrix: one action per time and size condition.
type ConflictRules struct {
	Time, Size    [3]RuleAction
	SkipIdentical bool
}



// Evaluate returns the action for a file: the matching time row if it is not Ask, else the
// matching size row, else Ask. Times compare at whole-second precision (SFTP resolution).
func (r ConflictRules) Evaluate(srcMod, dstMod time.Time, srcSize, dstSize int64) RuleAction {
	t := TimeSame
	switch s, d := srcMod.Unix(), dstMod.Unix(); {
	case d > s:
		t = TimeDestNewer
	case d < s:
		t = TimeDestOlder
	}
	z := SizeSame
	switch {
	case dstSize < srcSize:
		z = SizeDestSmaller
	case dstSize > srcSize:
		z = SizeDestLarger
	}
	if a := r.Time[t]; a != RuleAsk {
		return a
	}
	return r.Size[z]
}

// SizeChoiceEnabled reports whether action may be picked in size row: every size row overlaps
// every time row, so a non-Ask choice must not contradict a different non-Ask time action.
func (r ConflictRules) SizeChoiceEnabled(action RuleAction) bool {
	if action == RuleAsk {
		return true
	}
	for _, t := range r.Time {
		if t != RuleAsk && t != action {
			return false
		}
	}
	return true
}

// Normalize resets size choices that contradict the time rows to Ask.
func (r *ConflictRules) Normalize() {
	for i, a := range r.Size {
		if !r.SizeChoiceEnabled(a) {
			r.Size[i] = RuleAsk
		}
	}
}
