//go:build unix && !linux

package localfs

import (
	"syscall"
	"time"
)

// statChangeTime is Linux-only, like fileAccessTime: Stat_t's time fields differ per platform, and
// the Changed column stays blank elsewhere.
func statChangeTime(_ *syscall.Stat_t) time.Time { return time.Time{} }
