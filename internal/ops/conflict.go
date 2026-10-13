package ops

import (
	"fmt"
	"os"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// FileConflictFacts summarizes source and destination for a user conflict prompt.
type FileConflictFacts struct {
	Kind       string // "file" or "symlink" (the source)
	SourceSize int64
	SourceMod  time.Time
	DestSize   int64
	DestMod    time.Time
	// SourceTarget and DestTarget classify each side; a symlink is followed only to classify.
	SourceTarget PathTarget
	DestTarget   PathTarget
}

// PathTarget is what a conflict side ultimately is.
type PathTarget int

const (
	TargetOther   PathTarget = iota // file, symlink to a file, anything not a directory
	TargetDir                       // real directory
	TargetDirLink                   // symlink to a directory
)

// OverwriteUnsafe reports a collision overwrite must never resolve: it can only be skipped,
// renamed or canceled. That is a directory/non-directory mismatch, or two real directories (they
// merge before reaching a resolver, so this is only a safety net). Link onto link may be replaced.
func (f FileConflictFacts) OverwriteUnsafe() bool {
	sd, dd := f.SourceTarget != TargetOther, f.DestTarget != TargetOther
	if sd != dd {
		return true
	}
	return sd && (f.SourceTarget != TargetDirLink || f.DestTarget != TargetDirLink)
}

// DestIsDir reports whether the destination is a directory or a symlink to one.
func (f FileConflictFacts) DestIsDir() bool { return f.DestTarget != TargetOther }

func localTarget(path string, info os.FileInfo) PathTarget {
	if info.IsDir() {
		return TargetDir
	}
	if localfs.IsSymlink(info) {
		if st, err := os.Stat(path); err == nil && st.IsDir() {
			return TargetDirLink
		}
	}
	return TargetOther
}

// StatFileConflictFacts reads both paths for display in conflict UIs.
func StatFileConflictFacts(src, dst string) (FileConflictFacts, error) {
	si, err := os.Lstat(src)
	if err != nil {
		return FileConflictFacts{}, err
	}
	di, err := os.Lstat(dst)
	if err != nil {
		return FileConflictFacts{}, err
	}
	kind := "file"
	if localfs.IsSymlink(si) {
		kind = "symlink"
	}
	return FileConflictFacts{
		Kind:       kind,
		SourceSize: localfs.GetFileSize(si),
		SourceMod:  si.ModTime(),
		DestSize:   localfs.GetFileSize(di),
		DestMod:    di.ModTime(),

		SourceTarget: localTarget(src, si),
		DestTarget:   localTarget(dst, di),
	}, nil
}

// FormatConflictSize renders a byte size for conflict panels: decimal bytes plus a binary unit suffix.
func FormatConflictSize(n int64) string {
	if n < 0 {
		return "—"
	}
	return fmt.Sprintf("%d (%s)", n, formatByteSizeBinary(n))
}

func formatByteSizeBinary(n int64) string {
	const (
		KiB = int64(1024)
		MiB = KiB * 1024
		GiB = MiB * 1024
		TiB = GiB * 1024
	)
	switch {
	case n < KiB:
		return fmt.Sprintf("%d B", n)
	case n < MiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/float64(KiB))
	case n < GiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/float64(MiB))
	case n < TiB:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(GiB))
	default:
		return fmt.Sprintf("%.1f TiB", float64(n)/float64(TiB))
	}
}

// FormatConflictTime renders a modification time similar to classic file managers.
func FormatConflictTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 02 2006")
}
