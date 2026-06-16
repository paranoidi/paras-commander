package localfs

import (
	"io/fs"
	"time"
)

// FileTimes returns access and modification times for info.
// When the platform cannot read access time, ModTime is used for both.
func FileTimes(info fs.FileInfo) (access, mod time.Time) {
	mod = info.ModTime()
	return fileAccessTime(info, mod), mod
}
