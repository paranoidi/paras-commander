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
	// ExactDest makes Dest the target path of the single source (copy to a new file name, a
	// rename), never a directory the source goes into: a directory already at Dest is a
	// file-vs-directory conflict. Needs exactly one source.
	ExactDest bool
}

// FlattenJobRequest enqueues a flatten (move children + optional empty-dir cleanup) job.
// Roots are the selected directories; the job walks them while it runs.
type FlattenJobRequest struct {
	Roots       []string
	Dest        string
	Recursive   bool
	RemoveEmpty bool
}
