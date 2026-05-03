package jobs

// BlockerKind identifies why a running job is waiting for user input.
type BlockerKind string

const (
	BlockerKindConflict  BlockerKind = "conflict"
	BlockerKindDiskSpace BlockerKind = "disk-space"
)

// BlockerRequest is issued by transfer code when it needs a user decision before continuing.
type BlockerRequest struct {
	Kind      BlockerKind
	Conflict  *ConflictRequest
	DiskSpace *DiskSpaceBlockerRequest
}

// DiskSpaceBlockerRequest describes an insufficient-free-space condition on the destination volume.
type DiskSpaceBlockerRequest struct {
	Destination    string
	RequiredBytes  int64
	AvailableBytes uint64
	AvailableKnown bool
	NextSource     string
}

// BlockerDetails is a UI/event snapshot for an open blocker (mirrors BlockerRequest payloads).
type BlockerDetails struct {
	Kind      BlockerKind
	Conflict  *ConflictEvent
	DiskSpace *DiskSpaceBlockerDetails
}

// DiskSpaceBlockerDetails is the disk-space blocker shown in the jobs panel.
type DiskSpaceBlockerDetails struct {
	Destination    string
	RequiredBytes  int64
	AvailableBytes uint64
	AvailableKnown bool
	NextSource     string
}

// BlockerDetailsFromRequest builds display/event details from an in-flight request.
func BlockerDetailsFromRequest(req BlockerRequest) *BlockerDetails {
	switch req.Kind {
	case BlockerKindConflict:
		if req.Conflict == nil {
			return &BlockerDetails{Kind: BlockerKindConflict}
		}
		c := req.Conflict
		return &BlockerDetails{
			Kind: BlockerKindConflict,
			Conflict: &ConflictEvent{
				Source:          c.Source,
				Destination:     c.Destination,
				ExistingDetails: c.ExistingDetails,
				SourceSize:      c.SourceSize,
				SourceTime:      c.SourceTime,
				DestSize:        c.DestSize,
				DestTime:        c.DestTime,
			},
		}
	case BlockerKindDiskSpace:
		if req.DiskSpace == nil {
			return &BlockerDetails{Kind: BlockerKindDiskSpace}
		}
		d := req.DiskSpace
		return &BlockerDetails{
			Kind: BlockerKindDiskSpace,
			DiskSpace: &DiskSpaceBlockerDetails{
				Destination:    d.Destination,
				RequiredBytes:  d.RequiredBytes,
				AvailableBytes: d.AvailableBytes,
				AvailableKnown: d.AvailableKnown,
				NextSource:     d.NextSource,
			},
		}
	default:
		return &BlockerDetails{Kind: req.Kind}
	}
}
