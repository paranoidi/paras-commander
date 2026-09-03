package jobs

// ConflictDecision is the user's choice for resolving a file conflict.
type ConflictDecision string

const (
	DecisionOverwrite    ConflictDecision = "overwrite"
	DecisionSkip         ConflictDecision = "skip"
	DecisionOverwriteAll ConflictDecision = "overwrite-all"
	DecisionSkipAll      ConflictDecision = "skip-all"
	DecisionCancel       ConflictDecision = "cancel"
	// DecisionRetry applies to disk-space blockers only (re-check free space and continue).
	DecisionRetry ConflictDecision = "retry"
	// DecisionOverwriteAllSameSize applies to every remaining conflict in the job: overwrite when
	// source and destination sizes match, skip otherwise. No further prompting once selected.
	DecisionOverwriteAllSameSize ConflictDecision = "overwrite-all-same-size"
)

// ApplyAll reports whether the decision applies to all remaining conflicts
// in the current job.
func (d ConflictDecision) ApplyAll() bool {
	return d == DecisionOverwriteAll || d == DecisionSkipAll || d == DecisionOverwriteAllSameSize
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

// ShouldOverwrite reports whether the current policy says to overwrite.
func (p ConflictPolicy) ShouldOverwrite() bool {
	return p.activeDecision == DecisionOverwrite || p.activeDecision == DecisionOverwriteAll
}

// ShouldSkip reports whether the current policy says to skip.
func (p ConflictPolicy) ShouldSkip() bool {
	return p.activeDecision == DecisionSkip || p.activeDecision == DecisionSkipAll
}

// ApplyDecision determines the effective conflict outcome for a single file
// given the current policy and a new decision (which may be empty to reuse policy).
// It returns (shouldOverwrite, shouldSkip, shouldCancel, updatedPolicy).
func ApplyDecision(policy ConflictPolicy, newDecision ConflictDecision) (overwrite, skip, cancel bool, updated ConflictPolicy) {
	decision := newDecision
	if decision == "" {
		decision = policy.activeDecision
	}

	switch decision {
	case DecisionOverwrite:
		return true, false, false, policy
	case DecisionSkip:
		return false, true, false, policy
	case DecisionOverwriteAll:
		policy.activeDecision = DecisionOverwriteAll
		return true, false, false, policy
	case DecisionSkipAll:
		policy.activeDecision = DecisionSkipAll
		return false, true, false, policy
	case DecisionOverwriteAllSameSize:
		policy.activeDecision = DecisionOverwriteAllSameSize
		return true, false, false, policy
	case DecisionCancel:
		return false, false, true, policy
	case DecisionRetry:
		// Not a conflict outcome; must not be routed through ApplyDecision from conflict UI.
		return false, false, false, policy
	default:
		return false, false, false, policy
	}
}
