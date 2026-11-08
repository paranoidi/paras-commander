//go:build !unix

package localfs

import (
	"io/fs"
	"time"
)

type entryAttrs struct {
	owner, group          string
	links                 uint64
	changedAt, accessedAt time.Time
}

// entryAttrsFrom is unsupported off Unix; the file columns stay blank.
func entryAttrsFrom(_ fs.FileInfo) entryAttrs { return entryAttrs{} }
