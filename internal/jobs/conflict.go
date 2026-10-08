package jobs

import "strings"

// ConflictDecision is the user's choice for resolving a file conflict.
type ConflictDecision string

const (
	DecisionOverwrite ConflictDecision = "overwrite"
	DecisionSkip      ConflictDecision = "skip"
	DecisionCancel    ConflictDecision = "cancel"
	// DecisionRetry applies to disk-space blockers only (re-check free space and continue).
	DecisionRetry ConflictDecision = "retry"

	// Conditional rules from the "Overwrite advanced" dialog. The resolver evaluates them per file.
	DecisionOverwriteIfNewer           ConflictDecision = "overwrite-if-newer"
	DecisionOverwriteIfOlder           ConflictDecision = "overwrite-if-older"
	DecisionOverwriteIfExistingSmaller ConflictDecision = "overwrite-if-existing-smaller"
	DecisionOverwriteIfSizeDiffers     ConflictDecision = "overwrite-if-size-differs"
	DecisionOverwriteIfSameSize        ConflictDecision = "overwrite-if-same-size"
	// DecisionCompare skips identical files (removing the source on move); differing files prompt again.
	DecisionCompare ConflictDecision = "compare"
	// DecisionKeepBoth writes the new file under the first free "name (N).ext".
	DecisionKeepBoth ConflictDecision = "keep-both"

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
	// ContentDiffers marks a repeat prompt after Compare found different contents; the job's
	// apply-to-all policy is bypassed and Compare is not offered again.
	ContentDiffers bool
	// NoCompare hides Compare: the source is not comparable to the destination (archive extract).
	NoCompare bool
}

// ConflictPolicy tracks active overwrite/skip decisions within a job.
type ConflictPolicy struct {
	activeDecision ConflictDecision
}

// NewConflictPolicy creates a clean conflict policy with no active bulk decision.
func NewConflictPolicy() ConflictPolicy {
	return ConflictPolicy{}
}

// Decision returns the current active decision or empty string if none.
func (p ConflictPolicy) Decision() ConflictDecision {
	return p.activeDecision
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
