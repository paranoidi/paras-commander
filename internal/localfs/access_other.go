//go:build !unix

package localfs

import "io/fs"

// entryAccess is unsupported off Unix; the no-permission and broken-link marks are simply not shown.
func entryAccess(_ string, _ fs.FileInfo) (denied, brokenLink bool) {
	return false, false
}
