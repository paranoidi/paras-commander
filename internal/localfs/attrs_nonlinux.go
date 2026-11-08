//go:build !linux

package localfs

import "time"

// entryBirthTime needs statx; other platforms show a blank Created column.
func entryBirthTime(_ string) time.Time { return time.Time{} }
