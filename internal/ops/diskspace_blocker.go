package ops

import (
	"errors"

	"github.com/paranoidi/paras-commander/internal/fsvol"
	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// ErrUserCanceled is returned when the user aborts a disk-space wait.
var ErrUserCanceled = errors.New("canceled by user")

// DiskSpaceWaitDecision is the user's response to an insufficient-space prompt.
type DiskSpaceWaitDecision int

const (
	// DiskSpaceWaitRetry re-checks free space and continues.
	DiskSpaceWaitRetry DiskSpaceWaitDecision = iota
	// DiskSpaceWaitCancel aborts the transfer.
	DiskSpaceWaitCancel
)

// DiskSpaceWaitRequest describes an insufficient-free-space condition on the destination volume.
type DiskSpaceWaitRequest struct {
	Destination    string
	RequiredBytes  int64
	AvailableBytes uint64
	AvailableKnown bool
	NextSource     string
}

// DiskWaitFunc blocks until the destination volume has at least required bytes free,
// or the user aborts. Nil means disk-space prompts are disabled (tests / callers that skip checks).
type DiskWaitFunc func(req DiskSpaceWaitRequest) DiskSpaceWaitDecision

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
		switch wait(DiskSpaceWaitRequest{
			Destination:    destHost,
			RequiredBytes:  required,
			AvailableBytes: avail,
			AvailableKnown: ok,
			NextSource:     nextStr,
		}) {
		case DiskSpaceWaitCancel:
			return ErrUserCanceled
		case DiskSpaceWaitRetry:
			continue
		default:
			continue
		}
	}
}
