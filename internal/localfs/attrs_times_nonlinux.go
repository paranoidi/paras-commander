//go:build unix && !linux

package localfs

import (
	"syscall"
	"time"
)

// Stat_t field names differ per platform and only Linux is a build target; the columns stay blank.
func statChangeTime(_ *syscall.Stat_t) time.Time { return time.Time{} }

func statAccessTime(_ *syscall.Stat_t) time.Time { return time.Time{} }
