//go:build !linux

package findcrit

import "io/fs"

// MetaFromInfo builds an entry's Meta from its lstat result. Without Linux stat fields the
// change and access times fall back to the modification time and the owner is unknown (0).
func MetaFromInfo(info fs.FileInfo) *Meta {
	t := info.ModTime().UnixNano()
	return &Meta{ModTime: t, ChangeTime: t, AccessTime: t, Mode: info.Mode()}
}
