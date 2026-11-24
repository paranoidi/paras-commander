package pathloc

import "path/filepath"

// Panels, dialogs and handlers keep locations as strings that are host paths or scheme URIs
// (sftp://, any fileindex form). The functions here are the path/filepath operations for such a
// string: path/filepath itself would collapse the "//" of a URI. They do no I/O, and on a host
// path they are path/filepath.

// Clean is filepath.Clean of a host path, or the canonical form of a URI (as given when it does
// not parse).
func Clean(raw string) string {
	if !HasURIPrefix(raw) {
		return filepath.Clean(raw)
	}
	if p, err := Parse(raw); err == nil {
		return p.String()
	}
	return raw
}

// DiskPath is the cleaned disk path raw names: a host path itself, a fileindex path's host path.
// Any other URI has none and comes back as Clean gives it.
func DiskPath(raw string) string {
	if _, host, ok := CutIndexPrefix(raw); ok {
		return filepath.Clean(host)
	}
	return Clean(raw)
}
