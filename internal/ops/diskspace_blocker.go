package ops

import (
	"fmt"

	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/jobs"
)

// DiskWaitFunc blocks until the destination volume has at least required bytes free,
// or the user aborts. Nil means disk-space prompts are disabled (tests / callers that skip checks).
type DiskWaitFunc func(req jobs.BlockerRequest) jobs.ConflictDecision

// EnsureDiskSpace loops until required bytes are available on the volume containing destRoot,
// the host cannot measure free space (best-effort proceed), or the user aborts.
func EnsureDiskSpace(wait DiskWaitFunc, destRoot string, required int64, nextSource string) error {
	if wait == nil || required <= 0 {
		return nil
	}
	for {
		avail, _, ok := fsvol.VolumeBytes(destRoot)
		if !ok || int64(avail) >= required {
			return nil
		}
		switch wait(jobs.BlockerRequest{
			Kind: jobs.BlockerKindDiskSpace,
			DiskSpace: &jobs.DiskSpaceBlockerRequest{
				Destination:    destRoot,
				RequiredBytes:  required,
				AvailableBytes: avail,
				AvailableKnown: ok,
				NextSource:     nextSource,
			},
		}) {
		case jobs.DecisionCancel:
			return fmt.Errorf("canceled by user")
		case jobs.DecisionRetry:
			continue
		default:
			continue
		}
	}
}
