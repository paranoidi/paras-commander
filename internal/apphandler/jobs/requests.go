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
type FlattenJobRequest struct {
	Sources      []string
	Dest         string
	RemoveEmpty  bool
	FlattenRoots []string
}
