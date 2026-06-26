package ops

import (
	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// DiskWaitFunc blocks until the destination volume has at least required bytes free,
// or the user aborts. Nil means disk-space prompts are disabled (tests / callers that skip checks).
type DiskWaitFunc func(req jobs.BlockerRequest) jobs.ConflictDecision

// EnsureDiskSpace loops until required bytes are available on the volume containing destRoot,
// the host cannot measure free space (best-effort proceed), or the user aborts.
func EnsureDiskSpace(wait DiskWaitFunc, destRoot pathloc.Path, required int64, nextSource pathloc.Path) error {
	if wait == nil || required <= 0 || destRoot.IsRemote() {
		return nil
	}
	destHost, err := destRoot.FilePath()
	if err != nil {
		return err
	}
	nextStr := ""
	if !nextSource.IsZero() {
		nextStr = nextSource.String()
	}
	for {
		avail, _, ok := fsvol.VolumeBytes(destHost)
		if !ok || int64(avail) >= required {
			return nil
		}
		switch wait(jobs.BlockerRequest{
			Kind: jobs.BlockerKindDiskSpace,
			DiskSpace: &jobs.DiskSpaceBlockerRequest{
				Destination:    destHost,
				RequiredBytes:  required,
				AvailableBytes: avail,
				AvailableKnown: ok,
				NextSource:     nextStr,
			},
		}) {
		case jobs.DecisionCancel:
			return jobs.ErrUserCanceled
		case jobs.DecisionRetry:
			continue
		default:
			continue
		}
	}
}
