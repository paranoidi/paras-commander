package jobs

import "github.com/paranoidi/paras-commander/internal/jobs"

// TransferJobRequest enqueues a copy or move after scanning.
type TransferJobRequest struct {
	Type        jobs.Type
	Sources     []string
	Dest        string
	StartPaused bool
	Preserve    jobs.TransferPreserve
	// DestIsDir, when non-nil, is the destination's directory-ness already known to the caller
	// (a worker probe of a remote destination, or a directory the caller just created). Nil
	// means the enqueue decides it, which is false for a remote destination.
	DestIsDir *bool
}

// FlattenJobRequest enqueues a flatten (move children + optional empty-dir cleanup) job.
// Roots are the selected directories; the job walks them while it runs.
type FlattenJobRequest struct {
	Roots       []string
	Dest        string
	Recursive   bool
	RemoveEmpty bool
}
