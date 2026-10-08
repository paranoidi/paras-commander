package jobs

import "github.com/paranoidi/paras-commander/internal/jobs"

// TransferJobRequest enqueues a copy or move after scanning.
type TransferJobRequest struct {
	Type        jobs.Type
	Sources     []string
	Dest        string
	StartPaused bool
	Preserve    jobs.TransferPreserve
}

// FlattenJobRequest enqueues a flatten (move children + optional empty-dir cleanup) job.
// Roots are the selected directories; the job walks them while it runs.
type FlattenJobRequest struct {
	Roots       []string
	Dest        string
	Recursive   bool
	RemoveEmpty bool
}
