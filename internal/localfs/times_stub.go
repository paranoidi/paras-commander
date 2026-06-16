//go:build !linux

package localfs

import (
	"io/fs"
	"time"
)

func fileAccessTime(info fs.FileInfo, fallback time.Time) time.Time {
	return fallback
}
