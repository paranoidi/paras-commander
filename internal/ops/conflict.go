package ops

import (
	"fmt"
	"os"
	"time"

	"github.com/paranoidi/paras-commander/internal/localfs"
)

// FileConflictFacts summarizes source and destination for a user conflict prompt.
type FileConflictFacts struct {
	Kind       string // "file" or "symlink"
	SourceSize int64
	SourceMod  time.Time
	DestSize   int64
	DestMod    time.Time
}

// StatFileConflictFacts reads both paths for display in conflict UIs.
func StatFileConflictFacts(src, dst string) (FileConflictFacts, error) {
	si, err := os.Lstat(src)
	if err != nil {
		return FileConflictFacts{}, err
	}
	di, err := os.Stat(dst)
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
	}, nil
}

// FormatConflictSize renders a byte size for conflict panels (compact decimal).
func FormatConflictSize(n int64) string {
	return fmt.Sprintf("%d", n)
}

// FormatConflictTime renders a modification time similar to classic file managers.
func FormatConflictTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.Format("Jan 02 2006")
}
