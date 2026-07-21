//go:build !unix

package localfs

import "io/fs"

// entryAccessDenied is unsupported off Unix; the no-permission icon is simply not shown.
func entryAccessDenied(_ string, _ fs.FileInfo) bool {
	return false
}
