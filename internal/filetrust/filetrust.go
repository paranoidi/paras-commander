// Package filetrust decides whether a user-editable file whose contents become shell commands
// (meta.toml, menu.toml) is safe to load. The rule follows Midnight Commander's menu_file_own:
// the file must be owned by root or the current user, and nobody else may be able to write it.
package filetrust

import (
	"io/fs"
	"os"
)

// Reasons returned by Check.
const (
	ReasonNotRegular    = "not a regular file"
	ReasonOtherOwner    = "owned by another user"
	ReasonWorldWritable = "world-writable (chmod o-w)"
	ReasonGroupWritable = "group-writable by a shared group (chmod g-w)"
)

// Check returns "" when path is safe to load, otherwise a short reason it is not.
//
// Group write is accepted when the file's group is the owner's private group (a group named
// after the owning user, the usual Linux user-private-group setup with umask 0002): nobody else
// is in it, so it grants no one else write access.
func Check(path string) string {
	st, err := os.Lstat(path)
	if err != nil {
		return err.Error()
	}
	return checkInfo(st)
}

// evaluate is the platform-independent rule. privateGroup is only called for a group-writable
// file owned by root or euid.
func evaluate(mode fs.FileMode, uid, gid uint32, euid int, privateGroup func(uid, gid uint32) bool) string {
	if !mode.IsRegular() {
		return ReasonNotRegular
	}
	if uid != 0 && int64(uid) != int64(euid) {
		return ReasonOtherOwner
	}
	if mode.Perm()&0o002 != 0 {
		return ReasonWorldWritable
	}
	if mode.Perm()&0o020 != 0 && !privateGroup(uid, gid) {
		return ReasonGroupWritable
	}
	return ""
}
