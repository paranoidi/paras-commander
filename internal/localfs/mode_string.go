package localfs

import "io/fs"

// UnixModeString returns a Unix ls-style mode string for the file mode (e.g. "drwxr-xr-x").
func UnixModeString(mode fs.FileMode) string {
	return mode.String()
}
