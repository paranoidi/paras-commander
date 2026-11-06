package ops

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// Refusals of operations a fileindex:// panel (browse mode: the index only, ADR 0027) cannot do.
// The apphandler packages show them; this is their only definition.
const (
	IndexReadOnlyRefusal = "The fileindex is read-only: files cannot be added to it"
	IndexMoveOutRefusal  = "Moving out of the fileindex is not supported: copy instead"
	FlattenIndexRefusal  = "Flatten is not available on fileindex:// paths"
)

// OnDisk returns the file:// path a fileindex:// loc names when that path exists on disk (it is
// Lstat'ed, a symlink counts). A fileindex:// loc whose file is gone, and any other loc, comes
// back unchanged: the first is a record only the index backend may touch. Only the dedup view's
// delete uses it (Job.DeleteOnDisk, PreviewEmptyDirsUnder): everywhere else a fileindex:// path
// is the index's. It does disk I/O: call it from a worker.
func OnDisk(loc pathloc.Path) pathloc.Path {
	if loc.Scheme() != pathloc.SchemeFileIndex {
		return loc
	}
	local := loc.Local()
	host, err := local.FilePath()
	if err != nil {
		return loc
	}
	if _, err := os.Lstat(host); notThere(err) {
		return loc
	}
	return local
}

// OnDiskPaths applies OnDisk to each path.
func OnDiskPaths(locs []pathloc.Path) []pathloc.Path {
	out := make([]pathloc.Path, len(locs))
	for i, l := range locs {
		out[i] = OnDisk(l)
	}
	return out
}

// notThere reports whether an Lstat error means the path does not exist (a missing path or a
// missing directory above it) as opposed to one that cannot be examined.
func notThere(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}
