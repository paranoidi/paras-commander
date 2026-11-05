package ops

import (
	"errors"
	"io/fs"
	"os"
	"syscall"

	"github.com/paranoidi/paras-commander/internal/pathloc"
)

// OnDisk returns the file:// path a fileindex:// loc names when that path exists on disk (it is
// Lstat'ed, a symlink counts), so a delete, rename or cleanup acts on the real file and the index
// follows through the fschange sink. A fileindex:// loc whose file is gone, and any other loc,
// comes back unchanged: the first is a record only the index backend may touch, and the backend's
// listing (indexed files only) never decides what is removed from disk. It does disk I/O: call
// it from a worker.
func OnDisk(loc pathloc.Path) pathloc.Path {
	if loc.Scheme() != pathloc.SchemeFileIndex {
		return loc
	}
	local := loc.Local()
	host, err := local.FilePath()
	if err != nil {
		return loc
	}
	if _, err := os.Lstat(host); err != nil && notThere(err) {
		return loc
	}
	return local
}

// notThere reports whether an Lstat error means the path does not exist (a missing path or a
// missing directory above it) as opposed to one that cannot be examined.
func notThere(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// onDiskRoots translates fileindex:// roots with OnDisk and drops those that are not on disk,
// so cleanup walks only real directories and never the index backend.
func onDiskRoots(roots []pathloc.Path) []pathloc.Path {
	out := make([]pathloc.Path, 0, len(roots))
	for _, r := range roots {
		if r = OnDisk(r); r.Scheme() != pathloc.SchemeFileIndex {
			out = append(out, r)
		}
	}
	return out
}
