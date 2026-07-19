//go:build !unix

package localfs

import "io/fs"

// entryDevice is unsupported off Unix; mount-boundary icons are simply not shown.
func entryDevice(_ fs.FileInfo) (uint64, bool) {
	return 0, false
}
