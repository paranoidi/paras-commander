package localfs

import (
	"io"
	"io/fs"
	"os"
)

const execAnyMask fs.FileMode = 0111

const elfMagic = "\x7fELF"

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

// fileHasRunnableHeader reports whether the first bytes look like a script shebang or ELF binary.
func fileHasRunnableHeader(r io.Reader) (bool, error) {
	var hdr [4]byte
	n, err := io.ReadFull(r, hdr[:])
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return false, err
	}
	if n >= 2 && hdr[0] == '#' && hdr[1] == '!' {
		return true, nil
	}
	if n >= 4 && string(hdr[:4]) == elfMagic {
		return true, nil
	}
	return false, nil
}

// PathLooksRunnable stats path and reports whether Enter should exec it directly: POSIX +x regular
// file whose content starts with a shebang or ELF magic (avoids NAS/CIFS false +x on media files).
func PathLooksRunnable(path string) (bool, error) {
	exec, err := PathIsExecutable(path)
	if err != nil || !exec {
		return false, err
	}
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	return fileHasRunnableHeader(f)
}
