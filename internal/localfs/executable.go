package localfs

import (
	"io/fs"
	"os"
)

const execAnyMask fs.FileMode = 0111

// ModeIsExecutable reports whether mode is a regular file with any execute bit set.
func ModeIsExecutable(mode fs.FileMode) bool {
	return mode.IsRegular() && mode&execAnyMask != 0
}

// PathIsExecutable stats path and reports whether it is an executable regular file.
func PathIsExecutable(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return ModeIsExecutable(fi.Mode()), nil
}
